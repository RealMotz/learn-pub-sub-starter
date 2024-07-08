# learn-pub-sub-starter (Peril)

This is the starter code used in Boot.dev's [Learn Pub/Sub](https://learn.boot.dev/learn-pub-sub) course.

## Setup

1. Run `./rabbit start`
2. Add a new exchange called `peril_direct` of type direct
3. Add a new exchange called `peril_topic` of type topic
4. Add a new queue called `pause_test`
5. Add a new binding to `pause_test` with a routing key = `pause` (defined in our internal package) and `peril_direct` as the exchange
