package main

import (
	"fmt"
	"log"

	"github.com/streadway/amqp"
)

type Consumer struct {
	connection *amqp.Connection
	channel    *amqp.Channel
	tag        string
	done       chan error
}

func NewConsumer(amqpURI, exchange, exchangeType, queue, key, ctag string, exclusive, autoAck bool) (*Consumer, error) {
	c := &Consumer{
		connection: nil,
		channel:    nil,
		tag:        ctag,
		done:       make(chan error),
	}

	var err error

	log.Printf("Connecting to %s", amqpURI)
	c.connection, err = amqp.Dial(amqpURI)
	if err != nil {
		return nil, fmt.Errorf("Dial: %s", err)
	}

	log.Printf("Getting Channel")
	c.channel, err = c.connection.Channel()
	if err != nil {
		return nil, fmt.Errorf("channel: %s", err)
	}

	log.Printf("Declaring Exchange (%s)", exchange)
	if err = c.channel.ExchangeDeclare(
		exchange,     // name of the exchange
		exchangeType, // type
		true,         // durable
		false,        // delete when complete
		false,        // internal
		false,        // noWait
		nil,          // arguments
	); err != nil {
		return nil, fmt.Errorf("Exchange Declare: %s", err)
	}

	log.Printf("Declaring Queue (%s)", queue)
	state, err := c.channel.QueueDeclare(
		queue,     // name of the queue
		true,      // durable
		false,     // auto-delete
		exclusive, // exclusive
		false,     // noWait
		nil,       // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("Queue Declare: %s", err)
	}

	log.Printf("Declared Queue (%d messages, %d consumers), binding to Exchange (key '%s')", state.Messages, state.Consumers, key)
	if err = c.channel.QueueBind(
		queue,    // name of the queue
		key,      // routingKey
		exchange, // sourceExchange
		false,    // noWait
		nil,      // arguments
	); err != nil {
		return nil, fmt.Errorf("Queue Bind: %s", err)
	}

	log.Printf("Queue bound to Exchange, starting Consume (consumer tag '%s')", c.tag)
	deliveries, err := c.channel.Consume(
		queue,     // name
		c.tag,     // consumerTag,
		autoAck,   // autoAck
		exclusive, // exclusive
		false,     // noLocal
		false,     // noWait
		nil,       // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("Queue Consume: %s", err)
	}

	go handle(deliveries, c.done, c.channel)

	return c, nil
}

func (c *Consumer) Shutdown() error {
	// will close() the deliveries channel
	if err := c.channel.Cancel(c.tag, true); err != nil {
		return fmt.Errorf("Consumer cancel failed: %s", err)
	}

	if err := c.connection.Close(); err != nil {
		return fmt.Errorf("AMQP connection close error: %s", err)
	}

	defer log.Printf("AMQP shutdown OK")

	// wait for handle() to exit
	return <-c.done
}

func handle(deliveries <-chan amqp.Delivery, done chan error, ch *amqp.Channel) {
	for d := range deliveries {
		log.Printf(
			"Got %dB delivery: [%v] %s",
			len(d.Body),
			d.DeliveryTag,
			d.Body,
		)
		ch.Ack(d.DeliveryTag, false)
	}
	log.Printf("Handle: deliveries channel closed")
	done <- nil
}
