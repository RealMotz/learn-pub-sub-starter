package main

import (
	"fmt"
	"log"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.Acktype {
	return func(ps routing.PlayingState) pubsub.Acktype {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}

func handlerPlayerMove(gs *gamelogic.GameState, channel *amqp.Channel) func(gamelogic.ArmyMove) pubsub.Acktype {
	return func(move gamelogic.ArmyMove) pubsub.Acktype {
		defer fmt.Print("> ")
		outcome := gs.HandleMove(move)
		switch outcome {
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.Ack
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			err := pubsub.PublishJSON(
				channel,
				routing.ExchangePerilTopic,
				routing.WarRecognitionsPrefix+"."+gs.GetUsername(),
				gamelogic.RecognitionOfWar{
					Attacker: move.Player,
					Defender: gs.GetPlayerSnap(),
				},
			)
			if err != nil {
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		default:
			return pubsub.NackDiscard
		}
	}
}

func handlerWar(gs *gamelogic.GameState, channel *amqp.Channel) func(gamelogic.RecognitionOfWar) pubsub.Acktype {
	return func(rw gamelogic.RecognitionOfWar) pubsub.Acktype {
		defer fmt.Print("> ")
		outcome, winner, loser := gs.HandleWar(rw)
		var ack pubsub.Acktype
		var msg string

		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			ack = pubsub.Ack
			msg = fmt.Sprintf("%s won a against %s", winner, loser)
		case gamelogic.WarOutcomeYouWon:
			ack = pubsub.Ack
			msg = fmt.Sprintf("%s won a against %s", winner, loser)
		case gamelogic.WarOutcomeDraw:
			ack = pubsub.Ack
			msg = fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser)
		default:
			fmt.Println("Unhandled war outcome...")
			return pubsub.NackDiscard
		}

		key := routing.GameLogSlug + "." + rw.Attacker.Username
		pubsub.PublishGob(channel, routing.ExchangePerilTopic, key, msg)

		return ack
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

	queueName := fmt.Sprintf("%s.%s", routing.PauseKey, username)
	channel, _, err := pubsub.DeclareAndBind(
		conn,
		routing.ExchangePerilDirect,
		queueName,        // queueName
		routing.PauseKey, // key
		pubsub.Transient, // exchange type
	)
	if err != nil {
		log.Fatal(err)
	}

	gamestate := gamelogic.NewGameState(username)
	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilDirect,
		queueName,
		routing.PauseKey,
		pubsub.Transient,
		handlerPause(gamestate),
	)
	if err != nil {
		log.Fatalf("could not subscribe to pause: %v", err)
	}

	prefix := "army_moves"
	armyMoveQueueName := prefix + "." + username
	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		armyMoveQueueName, // queueName
		prefix+".*",       // key
		pubsub.Transient,  // exchange type
		handlerPlayerMove(gamestate, channel),
	)
	if err != nil {
		log.Fatalf("could not subscribe to army moves: %v", err)
	}

	err = pubsub.SubscribeJSON(
		conn,
		routing.ExchangePerilTopic,
		routing.WarRecognitionsPrefix,      // queueName
		routing.WarRecognitionsPrefix+".*", // key
		pubsub.Durable,                     // exchange type
		handlerWar(gamestate, channel),
	)
	if err != nil {
		log.Fatalf("could not subscribe to : %v", err)
	}

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
				armyMoveQueueName,
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
