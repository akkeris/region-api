package utils

import (
	structs "../structs"
	"encoding/json"
	"fmt"
	"gopkg.in/Shopify/sarama.v1"
	"os"
	"strings"
)

func SendToKafka(logentry structs.Logspec) (e error) {
	str, err := json.Marshal(logentry)
	if err != nil {
		fmt.Println("Error preparing request")
		return err
	}
	jsonStr := []byte(string(str))
	var brokersenv = os.Getenv("KAFKA_BROKERS")
	brokers := strings.Split(brokersenv, ",")

	producer, err := sarama.NewSyncProducer(brokers, nil)
	if err != nil {
		fmt.Println(err)
		return err
	}

	defer func() {
		if err := producer.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	msg := &sarama.ProducerMessage{Topic: logentry.Topic, Value: sarama.StringEncoder(jsonStr)}
	partition, offset, err := producer.SendMessage(msg)
	fmt.Println(partition)
	fmt.Println(offset)
	if err != nil {
		fmt.Println(err)
		return err
	} else {
		return nil
	}
}
