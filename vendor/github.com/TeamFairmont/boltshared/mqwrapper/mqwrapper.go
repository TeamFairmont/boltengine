// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package mqwrapper provides functions to create and utilize the MQ system as the BOLT engine
// requires. This includes default settings on Queues, Channels, etc.
// Tests require a server available at amqp://guest:guest@localhost:5672
package mqwrapper

import (
	"crypto/tls"
	"strings"

	"github.com/TeamFairmont/amqp"
	"github.com/TeamFairmont/gabs"
)

var closeCh = make(chan bool)

// Connection holds amqp connection info
type Connection struct {
	Connection *amqp.Connection
	Channel    *amqp.Channel
}

// Close closes the channel and connection
func (c *Connection) Close() {
	c.Channel.Close()
	c.Connection.Close()
}

// ConnectMQ connects to supplied RabbitMQ or other amqp url.
// Be sure to defer the .Close() calls on both the connection and channel
// Returns the Connection and an open Channel
func ConnectMQ(amqpURL string) (*Connection, error) {
	var conn *amqp.Connection
	var err error

	if strings.HasPrefix(amqpURL, "amqps") {
		conn, err = amqp.DialTLS(amqpURL, &tls.Config{MinVersion: tls.VersionTLS12})
	} else {
		conn, err = amqp.Dial(amqpURL)
	}

	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	return &Connection{conn, ch}, nil
}

// CreateConsumeTempQueue uses an open channel to create a temp queue for a command
// Returns the queue, the consume channel, and an error if either fails to create
func CreateConsumeTempQueue(ch *amqp.Channel) (*amqp.Queue, <-chan amqp.Delivery, error) {
	//setup queue to receive responses
	q, err := ch.QueueDeclare( //temp queue just for this command loop
		"",    // name
		false, // durable
		true,  // delete when usused
		true,  // exclusive
		false, // noWait
		nil,   // arguments
	)
	if err != nil {
		return nil, nil, err
	}

	res, err := ch.Consume(
		q.Name, // queue
		q.Name, // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return &q, nil, err
	}

	return &q, res, nil
}

//CloseRes trying to close res in workerStub
func CloseRes() {
	//recieved by a go routine in CreateConsumeNamedQueue, then closes ch
	closeCh <- true
}

// CreateConsumeNamedQueue uses an open channel to create a durable named queue
// Returns the queue, the consume channel, and an error if either fails to create
func CreateConsumeNamedQueue(name string, ch *amqp.Channel) (*amqp.Queue, <-chan amqp.Delivery, error) {
	//setup queue to receive responses
	q, err := ch.QueueDeclare(
		name,  // name
		true,  // durable
		false, // delete when usused
		false, // exclusive
		false, // noWait
		nil,   // arguments
	)
	if err != nil {
		return nil, nil, err
	}

	res, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return &q, nil, err
	}

	//This is needed to close a go routine in workerStub
	//when CloseRes() is called
	go func() {
		<-closeCh
		ch.Close()
	}()
	return &q, res, nil

}

// PublishCommand pushes a processing command up to the MQ with its uuid, command name, and payload
// The replyTo should normally be set to the queue.Name created via CreateConsumeTempQueue
func PublishCommand(q *amqp.Channel, id string, prefix string, command string, payload *gabs.Container, replyTo string) error {
	pub := amqp.Publishing{
		DeliveryMode:  amqp.Persistent,
		ContentType:   "text/json",
		Body:          []byte(payload.String()),
		CorrelationId: id,
	}
	if replyTo != "" {
		pub.ReplyTo = replyTo
	}

	return q.Publish(
		"",             // exchange
		prefix+command, // routing key
		false,          // mandatory
		false,
		pub)
}
