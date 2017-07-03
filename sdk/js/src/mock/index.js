// Copyright 2017 Capsule8 Inc. All rights reserved.

import MockSubscription from "./subscription";
import { EVENTS } from "./events";

// Thin wrapper around the node ws lib to handle grabbing events
class MockClient {
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
    const heartbeat = setInterval(
      () => {
        // Sample event
        const i = getRandomIntInclusive(0, EVENTS.length - 1);
        msgHandlerFn(EVENTS[i]);
      },
      this._heartbeatInterval,
    );
    return new MockSubscription(null, signedSubscription, heartbeat);
  }
};

// Thanks mozilla
const getRandomIntInclusive = (min, max) => {
    min = Math.ceil(min);
      max = Math.floor(max);
        return Math.floor(Math.random() * (max - min + 1)) + min;
}

export default MockClient;
