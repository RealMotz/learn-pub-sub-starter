package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	Durable = iota + 1
	Transient
)

type Acktype int

const (
	Ack Acktype = iota + 1
	NackRequeue
	NackDiscard
)

func PublishGob[T any](ch *amqp.Channel, exchange, key string, val T) error {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	err := encoder.Encode(val)
	if err != nil {
		return err
	}

	ctx := context.Background()
	ch.PublishWithContext(ctx, exchange, key, false, false, amqp.Publishing{
		ContentType: "application/gob",
		Body:        buffer.Bytes(),
	})
	return nil
}

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}

	ctx := context.Background()
	ch.PublishWithContext(ctx, exchange, key, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        data,
	})

	return nil
}

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType int, // an enum to represent "durable" or "transient"
) (*amqp.Channel, amqp.Queue, error) {
	channel, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("Could not create a RabbitMQ channel: %v", err)
	}

	isDurable := simpleQueueType == Durable
	table := amqp.Table{"x-dead-letter-exchange": "peril_dlx"}
	queue, err := channel.QueueDeclare(queueName, isDurable, !isDurable, !isDurable, false, table)
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("Could not create a RabbitMQ queue: %v", err)
	}

	err = channel.QueueBind(queue.Name, key, exchange, false, nil)
	if err != nil {
		return nil, amqp.Queue{}, fmt.Errorf("could not bind queue: %v", err)
	}

	return channel, queue, nil
}

func SubscribeGob[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType int,
	handler func(T) Acktype,
) error {
	unmarshal := func(data []byte) (T, error) {
		buffer := bytes.NewBuffer(data)
		decoder := gob.NewDecoder(buffer)
		var out T
		err := decoder.Decode(&out)
		if err != nil {
			fmt.Printf("Error unmarshaling message: %v", err)
			return out, err
		}

		return out, nil
	}

	return subscribe(conn, exchange, queueName, key, simpleQueueType, handler, unmarshal)
}

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType int,
	handler func(T) Acktype,
) error {
	// channel, _, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	// if err != nil {
	// 	return err
	// }

	unmarshal := func(data []byte) (T, error) {
		var out T
		err := json.Unmarshal(data, &out)
		if err != nil {
			fmt.Printf("Error unmarshaling message: %v", err)
			return out, err
		}

		return out, nil
	}

	return subscribe(conn, exchange, queueName, key, simpleQueueType, handler, unmarshal)

	// var messages <-chan amqp.Delivery
	// messages, err = channel.Consume(queueName, "", false, false, false, false, nil)
	// go func() {
	// 	defer channel.Close()
	// 	for msg := range messages {
	// 		var out T
	// 		err := json.Unmarshal(msg.Body, &out)
	// 		if err != nil {
	// 			fmt.Printf("Error unmarshaling message: %v", err)
	// 		}
	// 		fmt.Println(out)
	// 		ack := handler(out)

	// 		if ack == NackDiscard {
	// 			msg.Nack(false, false)
	// 		} else if ack == NackRequeue {
	// 			msg.Nack(false, true)
	// 		} else {
	// 			msg.Ack(false)
	// 		}
	// 	}
	// }()

	// return nil
}

func subscribe[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType int,
	handler func(T) Acktype,
	unmarshaller func([]byte) (T, error),
) error {
	channel, _, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	if err != nil {
		return err
	}

	var messages <-chan amqp.Delivery
	messages, err = channel.Consume(queueName, "", false, false, false, false, nil)
	go func() {
		defer channel.Close()
		for msg := range messages {
			out, err := unmarshaller(msg.Body)
			if err != nil {
				fmt.Printf("Error unmarshaling message: %v", err)
			}
			ack := handler(out)

			if ack == NackDiscard {
				msg.Nack(false, false)
			} else if ack == NackRequeue {
				msg.Nack(false, true)
			} else {
				msg.Ack(false)
			}
		}
	}()

	return nil
}
