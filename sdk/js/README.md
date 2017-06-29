# JS Client SDK

Javascript client SDK.

## Getting Started
Install node 6.11.0 (LTS)
```
$ curl -sL https://deb.nodesource.com/setup_6.x | sudo -E bash -
$ sudo apt-get install nodejs
```

## Dependencies
 * NodeJS version 6.11.0
 * Everything in `package.json`

## Usage
```
// Pass in the base url of the grpc gateway proxy API
const client = require("client")("localhost:8081/v0");

const subscription = client.getEvents({
    "subscription": {
        "selector": {
            "chargen": {
                "length": 1,
            },
        },
        "modifier": {
            "throttle": {
                "interval": 1,
                "interval_type": 1,
            },
        },
    },
    "subscription_id": "somehash",
    "signature": "somesignature",
// Pass in a msg handler function to handle incoming events
}, (event) => {
    // do something w/ `event`
});

// Finished? Clean up the subscription.
subscription.close();
```
