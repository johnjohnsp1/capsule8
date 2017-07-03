// Copyright 2017 Capsule8 Inc. All rights reserved.

// Status codes as per RFC6455 (https://tools.ietf.org/html/rfc6455#section-7.4.1)
const NORMAL_CLOSE = 1000;

class Subscription {
  constructor(client, signedSubscription, heartbeat) {
    this._client = client;
    this._subscription = signedSubscription;
    this._heartbeat = heartbeat;
  }

  close() {
    clearInterval(this._heartbeat);
    this._client.close(NORMAL_CLOSE, "client has closed the subscription");
  }
}

export default Subscription;
