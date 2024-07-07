package main

import (
	"fmt"
	"log"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func subscribeToPauseQueue(conn *amqp.Connection, gs *gamelogic.GameState, username string) {
	err := pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilDirect,
		routing.PauseKey+"."+username, // queueName
		routing.PauseKey,              // key
		pubsub.Transient,              // exchange type
		handlerPause(gs),
	)
	if err != nil {
		log.Fatalf("could not subscribe to pause: %v", err)
	}
}

func subscribeToMoveQueue(conn *amqp.Connection, gs *gamelogic.GameState, channel *amqp.Channel, username string) {
	err := pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		routing.ArmyMovesPrefix+"."+username, // queueName
		routing.ArmyMovesPrefix+".*",         // key
		pubsub.Transient,                     // exchange type
		handlerPlayerMove(gs, channel),
	)
	if err != nil {
		log.Fatalf("could not subscribe to army moves: %v", err)
	}
}

func subscribeToWarQueue(conn *amqp.Connection, gs *gamelogic.GameState, channel *amqp.Channel, username string) {
	err := pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		routing.WarRecognitionsPrefix,      // queueName
		routing.WarRecognitionsPrefix+".*", // key
		pubsub.Durable,                     // exchange type
		handlerWar(gs, channel),
	)
	if err != nil {
		log.Fatalf("could not subscribe to : %v", err)
	}
}

func main() {
	fmt.Println("Starting Peril client... 🤩")
	connectionStr := "amqp://guest:guest@localhost:5672/"
	conn, err := amqp.Dial(connectionStr)

	if err != nil {
		log.Fatalf("Could not connect to RabbitMQ: %v", err)
	}

	defer conn.Close()
	fmt.Println("Connection successful 😎")
	username, err := gamelogic.ClientWelcome()

	if err != nil {
		log.Fatal(err)
	}

	channel, _, err := pubsub.DeclareAndBind(
		conn,
		routing.ExchangePerilDirect,
		routing.PauseKey+"."+username, // queueName
		routing.PauseKey,              // key
		pubsub.Transient,              // exchange type
	)
	if err != nil {
		log.Fatal(err)
	}

	gamestate := gamelogic.NewGameState(username)
	subscribeToPauseQueue(conn, gamestate, username)
	subscribeToMoveQueue(conn, gamestate, channel, username)
	subscribeToWarQueue(conn, gamestate, channel, username)

	for {
		cmd := gamelogic.GetInput()
		if len(cmd) == 0 {
			continue
		}

		switch cmd[0] {
		case "spawn":
			err := gamestate.CommandSpawn(cmd)
			if err != nil {
				fmt.Printf("Error spawning units: %v\n", err)
			}
		case "move":
			mv, err := gamestate.CommandMove(cmd)
			if err != nil {
				fmt.Printf("Error spawning units: %v\n", err)
			}
			err = pubsub.PublishJSON(
				channel,
				routing.ExchangePerilTopic,
				routing.ArmyMovesPrefix+"."+username,
				mv,
			)
			if err != nil {
				fmt.Printf("Error publishing move: %v\n", err)
				continue
			}
			fmt.Println("Successfully moved unit!")
		case "status":
			gamestate.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			fmt.Println("Spamming not allowed yet!")
		case "quit":
			gamelogic.PrintQuit()
			fmt.Println("\nShutting client down... ❌")
			return
		default:
			fmt.Println("unknown command")
		}
	}
}
