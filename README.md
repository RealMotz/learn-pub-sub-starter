# learn-pub-sub-starter (Peril)

This is the starter code used in Boot.dev's [Learn Pub/Sub](https://learn.boot.dev/learn-pub-sub) course.

## Setup

1. Run `./rabbit start`
2. Add a new exchange called `peril_direct`
3. Add a new queue called `pause_test`
4. Add a new binding to our queue with a routing key = `pause` (defined in our internal package)
