package main

import (
	"fmt"
	"log"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func handlerLogs() func(routing.GameLog) pubsub.Acktype {
	return func(gamelog routing.GameLog) pubsub.Acktype {
		defer fmt.Print("> ")
		gamelogic.WriteLog(gamelog)
		return pubsub.Ack
	}
}

func subscribeToLogs(conn *amqp.Connection) {
	err := pubsub.SubscribeGob(
		conn,
		routing.ExchangePerilTopic,
		routing.GameLogSlug,
		routing.GameLogSlug+".*",
		pubsub.Durable,
		handlerLogs(),
	)
	if err != nil {
		log.Fatalf("could not subscribe to logs: %v", err)
	}
}

func main() {
	fmt.Println("Starting Peril server... 🏃")
	connectionStr := "amqp://guest:guest@localhost:5672/"

	conn, err := amqp.Dial(connectionStr)
	if err != nil {
		log.Fatalf("Could not connect to RabbitMQ: %v", err)
	}
	defer conn.Close()
	fmt.Println("Connection successful 👍")

	channel, err := conn.Channel()
	if err != nil {
		log.Fatalf("Could not create an RabbitMQ channel: %v", err)
	}

	_, _, err = pubsub.DeclareAndBind(
		conn,
		routing.ExchangePerilTopic,
		routing.GameLogSlug,
		routing.GameLogSlug+".*",
		pubsub.Durable,
	)
	if err != nil {
		fmt.Println("this?")
		log.Fatalf("Could not bind channel to queue: %v", err)
	}

	subscribeToLogs(conn)
	gamelogic.PrintServerHelp()
	for {
		input := gamelogic.GetInput()
		if len(input) == 0 {
			continue
		}

		cmd := input[0]

		if cmd == "quit" {
			fmt.Println("exiting...")
			break
		} else if cmd == "pause" {
			fmt.Println("Sending pause message...")
			pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true})
		} else if cmd == "resume" {
			fmt.Println("Sending resume message...")
			pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: false})
		} else {
			fmt.Println("command not found")
		}
	}

	fmt.Println("\nShutting down... ❌")
}
