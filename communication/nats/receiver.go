/*
 * Copyright (C) 2017 The "MysteriumNetwork/node" Authors.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package nats

import (
	"sync"

	"github.com/mysteriumnetwork/node/communication"
	"github.com/mysteriumnetwork/node/logconfig"
	"github.com/nats-io/go-nats"
	"github.com/pkg/errors"
)

var rlog = logconfig.NewNamespaceLogger("receiver")

// NewReceiver constructs new Receiver's instance which works through NATS connection.
// Codec packs/unpacks messages to byte payloads.
// Topic (optional) if need to send messages prefixed topic.
func NewReceiver(connection Connection, codec communication.Codec, topic string) *receiverNATS {
	return &receiverNATS{
		connection:   connection,
		codec:        codec,
		messageTopic: topic + ".",
		subs:         make(map[string]*nats.Subscription),
	}
}

type receiverNATS struct {
	connection   Connection
	codec        communication.Codec
	messageTopic string

	mu   sync.Mutex
	subs map[string]*nats.Subscription
}

func (receiver *receiverNATS) Receive(consumer communication.MessageConsumer) error {
	messageTopic := receiver.messageTopic + string(consumer.GetMessageEndpoint())

	messageHandler := func(msg *nats.Msg) {
		rlog.Debugf("message %q received: %s", messageTopic, msg.Data)
		messagePtr := consumer.NewMessage()
		err := receiver.codec.Unpack(msg.Data, messagePtr)
		if err != nil {
			err = errors.Wrapf(err, "failed to unpack message %q", messageTopic)
			rlog.Error(err)
			return
		}

		err = consumer.Consume(messagePtr)
		if err != nil {
			err = errors.Wrapf(err, "failed to process message %q", messageTopic)
			rlog.Error(err)
			return
		}
	}

	receiver.mu.Lock()
	defer receiver.mu.Unlock()

	subscription, err := receiver.connection.Subscribe(messageTopic, messageHandler)
	if err != nil {
		err = errors.Wrapf(err, "failed subscribe message '%s'", messageTopic)
		return err
	}
	receiver.subs[messageTopic] = subscription
	return nil
}

func (receiver *receiverNATS) Unsubscribe() {
	receiver.mu.Lock()
	defer receiver.mu.Unlock()

	for topic, s := range receiver.subs {
		if err := s.Unsubscribe(); err != nil {
			rlog.Error("failed to unsubscribe from topic: ", topic)
		}
		rlog.Infof(" unsubscribed from %s", topic)
	}
}

func (receiver *receiverNATS) Respond(consumer communication.RequestConsumer) error {
	requestTopic := receiver.messageTopic + string(consumer.GetRequestEndpoint())

	messageHandler := func(msg *nats.Msg) {
		rlog.Debugf("request %q received: %s", requestTopic, msg.Data)
		requestPtr := consumer.NewRequest()
		err := receiver.codec.Unpack(msg.Data, requestPtr)
		if err != nil {
			err = errors.Wrapf(err, "failed to unpack request '%s'", requestTopic)
			rlog.Error(err)
			return
		}

		response, err := consumer.Consume(requestPtr)
		if err != nil {
			err = errors.Wrapf(err, "failed to process request '%s'", requestTopic)
			rlog.Error(err)
			return
		}

		responseData, err := receiver.codec.Pack(response)
		if err != nil {
			err = errors.Wrapf(err, "failed to pack response '%s'", requestTopic)
			rlog.Error(err)
			return
		}

		rlog.Debugf("request %q response: %s", requestTopic, responseData)
		err = receiver.connection.Publish(msg.Reply, responseData)
		if err != nil {
			err = errors.Wrapf(err, "failed to send response '%s'", requestTopic)
			rlog.Error(err)
			return
		}
	}

	receiver.mu.Lock()
	defer receiver.mu.Unlock()

	if subscription, ok := receiver.subs[requestTopic]; ok && subscription.IsValid() {
		rlog.Debugf("already subscribed to %q topic", requestTopic)
		return nil
	}

	rlog.Debugf("request %q topic has been subscribed to", requestTopic)

	subscription, err := receiver.connection.Subscribe(requestTopic, messageHandler)
	if err != nil {
		err = errors.Wrapf(err, "failed subscribe request '%s'", requestTopic)
		return err
	}

	receiver.subs[requestTopic] = subscription
	return nil
}
