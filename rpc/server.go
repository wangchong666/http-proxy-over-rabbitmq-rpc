package rpc

import (
	"io"
	"fmt"
	"github.com/streadway/amqp"
	"net"
	"bytes"
	log "github.com/sirupsen/logrus"
)


func Proxy(conf *Config){
	l, err := net.Listen("tcp",  fmt.Sprintf(":%d", conf.Port))
	if err != nil {
		log.Panic(err)
	}

	conn, err := amqp.Dial(conf.Amqp)
	failOnError(err, "Failed to connect to RabbitMQ")

	for {
		client, err := l.Accept()
		if err != nil {
			log.Panic(err)
		}
		// client.SetReadDeadline(time.Now().Add(5 * time.Second))
		go handleClientRequest(client,conn,conf)
	}
}

func handleClientRequest(client net.Conn,conn *amqp.Connection,conf *Config) {
	if client == nil {
		return
	}
	// defer client.Close()

	rio,err := Dial(conn , conf)
	failOnError(err,"cannot connect to mq")

	rio.conn = client
	defer rio.Close()


	wait := make(chan bool)
	//进行转发
	go func(){
		io.Copy(rio, client)
		log.Debug("1完成")
		wait<-true
	}()
	go func(){
		io.Copy(client, rio)
		log.Debug("2完成")
		wait<-true
	}()
	
	<-wait
	log.Debug("关闭。。。")

}

func Dial(conn *amqp.Connection,conf *Config) (*rabbitIO, error) {

	// defer conn.Close()

	ch, err := conn.Channel()
	if(err != nil){
		log.Error("Failed to open a channel",err)
		return nil,err
	}
	// failOnError(err, "Failed to open a channel")
	// defer ch.Close()

	q, err := ch.QueueDeclare(
		"",    // name
		false, // durable
		true, // delete when unused
		true,  // exclusive
		false, // noWait
		nil,   // arguments
	)
	failOnError(err, "Failed to declare a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	corrId := randomString(32)

	io := &rabbitIO{
		correlationId:corrId,
		ch:ch,
		q:q,
		msgs: msgs,
		sender: true,
		queueName:conf.QueueName,
	}
	log.Info("new connection",corrId)
	return io,nil
}



func Accept(conf *Config, count int){
	conn, err := amqp.Dial(conf.Amqp)
	failOnError(err, "Failed to open a amqp connection")
	for i:=0;i<count;i++ {
		go accept(conf,conn)
	}
	forever := make(chan bool)
	<-forever
}

var conn_map = make(map[string]*rabbitIO) //连接复用
func accept(conf *Config,conn *amqp.Connection) {
	ch, err := conn.Channel()
	defer ch.Close()
	failOnError(err, "Failed to open a channel")
	q, err := ch.QueueDeclare(
		conf.QueueName, // name
		false,       // durable
		false,       // delete when unused
		false,       // exclusive
		false,       // no-wait
		nil,         // arguments
	)
	failOnError(err, "Failed to declare a queue")

	err = ch.Qos(
		1,     // prefetch count
		0,     // prefetch size
		false, // global
	)
	failOnError(err, "Failed to set QoS")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")
    
	log.Printf(" [*] Awaiting RPC requests")
	for d := range msgs {
		log.Debug("read ",len(d.Body)," bytes")

		c,ok := conn_map[d.CorrelationId]
		// log.Debug(d.CorrelationId,ok)
		if(!ok){
			c := &rabbitIO{
				correlationId:d.CorrelationId,
				ch:ch,
				q:q,
				msgs: msgs,
				sender:false,
				replyTo:d.ReplyTo,
				queueName:conf.QueueName,
				buffer:bytes.NewBuffer([]byte{}),
			}
			conn_map[d.CorrelationId] = c
			c.replyTo = d.ReplyTo
			c.correlationId = d.CorrelationId
			
			go c.CreateRequest(d.Body)
			// copy(b[0:10], d.Body[0:10])
		}else{
			go func(){
				if(c.conn != nil){
					io.Copy(c.conn,c.buffer)
					c.conn.Write(d.Body)
				}else{
					c.buffer.Write(d.Body)
				}
			}()
		}
		d.Ack(true)
	}
	forever := make(chan bool)
	<-forever
}

