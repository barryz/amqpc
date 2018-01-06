package main

import (
	"crypto/sha1"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

const (
	DEFAULT_EXCHANGE      string = " "
	DEFAULT_EXCHANGE_TYPE string = "direct"
	DEFAULT_QUEUE         string = "amqpc-queue"
	DEFAULT_ROUTING_KEY   string = "amqpc-key"
	DEFAULT_CONSUMER_TAG  string = "amqpc-consumer"
	DEFAULT_RELIABLE      bool   = true
	DEFAULT_INTERVAL      int    = 500
	DEFAULT_MESSAGE_COUNT int    = 0
	DEFAULT_CONCURRENCY   int    = 1
)

var (
	exchange   *string
	routingKey *string
	queue      *string
	body       *string
)

// Flags
var (
	consumer = flag.Bool("c", true, "Act as a consumer")
	producer = flag.Bool("p", false, "Act as a producer")

	// RabbitMQ related
	uri          = flag.String("u", "amqp://test:test@localhost:5772/test_vhost", "AMQP URI")
	exchangeType = flag.String("t", DEFAULT_EXCHANGE_TYPE, "Exchange type - direct|fanout|topic|x-custom")
	consumerTag  = flag.String("ct", DEFAULT_CONSUMER_TAG, "AMQP consumer tag (should not be blank)")
	reliable     = flag.Bool("r", DEFAULT_RELIABLE, "Wait for the publisher confirmation before exiting")

	// Test bench related
	concurrency  = flag.Int("g", DEFAULT_CONCURRENCY, "Concurrency")
	interval     = flag.Int("i", DEFAULT_INTERVAL, "(Producer only) Interval at which send messages (in ms)")
	messageCount = flag.Int("n", DEFAULT_MESSAGE_COUNT, "(Producer only) Number of messages to send")

	// Consume Flags
	excl    = flag.Bool("excl", false, "Exclusive queue")
	autoAck = flag.Bool("aack", false, "Auto Ack")
)

func usage() {
	fmt.Fprintf(os.Stderr, "amqpc is CLI tool for testing AMQP brokers\n")
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  Consumer : amqpc [options] -c exchange routingkey queue\n")
	fmt.Fprintf(os.Stderr, "  Producer : amqpc [options] -p exchange routingkey [message]\n")
	fmt.Fprintf(os.Stderr, "\nOptions:\n")
	flag.PrintDefaults()
	os.Exit(1)
}

func init() {
	flag.Usage = usage
	flag.Parse()
}

func main() {
	done := make(chan error)

	flag.Usage = usage
	args := flag.Args()

	if flag.NArg() != 3 {
		log.Printf("You're missing arguments")
		os.Exit(1)
	}

	exchange = &args[0]
	routingKey = &args[1]

	if *producer {
		body = &args[2]
		for i := 0; i < *concurrency; i++ {
			go startProducer(done, body, *messageCount, *interval)
		}
	} else {
		if *concurrency > 1 && *excl {
			log.Fatal("consume exclusive queue only need one connection!")
		}
		queue = &args[2]
		for i := 0; i < *concurrency; i++ {
			go startConsumer(done)
		}
	}

	err := <-done
	if err != nil {
		log.Fatalf("Error : %s", err)
	}

	log.Printf("Exiting...")
}

func startConsumer(done chan error) {
	_, err := NewConsumer(
		*uri,
		*exchange,
		*exchangeType,
		*queue,
		*routingKey,
		*consumerTag,
		*excl,
		*autoAck,
	)

	if err != nil {
		log.Fatalf("Error while starting consumer : %s", err)
	}

	<-done
}

func startProducer(done chan error, body *string, messageCount, interval int) {
	p, err := NewProducer(
		*uri,
		*exchange,
		*exchangeType,
		*routingKey,
		*consumerTag,
		true,
	)

	if err != nil {
		log.Fatalf("Error while starting producer : %s", err)
	}

	var i int = 1
	for messageCount != 0 && i < messageCount {
		publish(p, body, i)
		i++
		time.Sleep(time.Duration(interval) * time.Millisecond)
	}

	done <- nil
}

func publish(p *Producer, body *string, i int) {
	// Generate SHA for body
	hasher := sha1.New()
	hasher.Write([]byte(*body + string(i)))
	sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))

	// String to publish
	bodyString := fmt.Sprintf("body: %s - hash: %s", *body, sha)

	if err := p.Publish(*exchange, *routingKey, bodyString); err != nil {
		log.Printf("%v\n", err)
	}
}
