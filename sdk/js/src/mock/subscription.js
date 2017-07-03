// Copyright 2017 Capsule8 Inc. All rights reserved.

// Status codes as per RFC6455 (https://tools.ietf.org/html/rfc6455#section-7.4.1)
const NORMAL_CLOSE = 1000;

class MockSubscription {
  constructor(client, signedSubscription, heartbeat) {
    this._client = client;
    this._subscription = signedSubscription;
    this._heartbeat = heartbeat;
  }

  close() {}
}

export default MockSubscription;
