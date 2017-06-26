// Copyright 2017 Capsule8 Inc. All rights reserved.

const path = require("path");

const WebSocket = require("ws");
const axios = require("axios");

const Subscription = require("./subscription");

// Thin wrapper around the node ws lib to handle grabbing events
class Client {
  constructor(baseUrl) {
    this._baseUrl = baseUrl;
    // Heartbeat interval in milliseconds
    this._heartbeatInterval = 1000;
  }

  // getEvents gets events from the API
  // ARGS:
  //    signedSubscription - signed subscription object
  //    msgHandlerFn - handler function for incoming data
  // RETURNS:
  //    subscription object
  getEvents(signedSubscription, msgHandlerFn) {
    const client = new WebSocket(path.join(this._baseUrl, "topic", "event." + signedSubscription.subscription_id));
    client.on("message", msgHandlerFn);
    const heartbeat = setInterval(
      () => {
        // Publish heartbeats at an interval
        axios.post(path.join(this._baseUrl, "topic", "subscription." + signedSubscription.subscription_id),
          signedSubscription
        ).catch((err) => {
          // TODO: Should we do something here or is fire and forget semantics okay?
        });
      },
      this._heartbeatInterval,
    );
    return new Subscription(client, signedSubscription, heartbeat);
  }
};

module.exports = Client;
