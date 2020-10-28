### Example Reverse Communication Client / Server

First, run the server. From the base directory,
```
$ cd server
$ go run main.go
```

The server will run by default on port 7777, but this is configurable like so:
```
$ go run main.go --port=8888
```

Second run `n` number of clients. From the base directory,
```
$ cd client
$ go run main.go
```

Defaults to making requests to `localhost:7777`, but can also be configured:
```
$ go run main.go --port=80 --host=example.com 
```

The client will print a pair of messages:
```
2020/10/27 20:56:43 Client id <UUID> is running, looking for a server at <host>:<port>
2020/10/27 20:56:43 A GET request to http://<host>:<port>/clients/<UUID>/system-time will result in reporting this client's system time
```

One can copy the URL printed in the second message and use cURL or Postman to retrieve the timestamp. The response to that route will look like:

```
{
    error: bool,
    msg: "a string indicating the nature of the error if error is true"
    ts: "a string of the client's timestamp"
}
```

Expanding the `checkInResp` structure to contain additional fields would allow more complex commands to be issued to the client. These additional commands would require addtional routes to be added in the server's `main` function, and possibly additional in memory channel maps, like the `clientTimeMap` and the `clientCommandMap` along with their respective locks.

To issue additional commands, a `command` struct could be created and used instead of an `interface{}`, or additional channels could be used like semaphores, each indicating a different action that the client should take.

If significantly expanding the command mechanisms, consider containing the maps in their own go-routine, and rather than contesting locks, pass keys to the map as a message over a channel, and receive the value (a channel to a given client / request) over a channel.

#### Testing
Testing exists for the functions underlying the server routes. To run the tests, from the top-level of the repo:
```
$ cd server
$ go test
```
It will take a little over a minute to run to test the long-polling and time out functionality.

#### Next Steps
There are several structs used by both `main.go` files which should be broken into a `lib` or `util` package and imported by both, rather than copied+pasted into each.

The client's functionality is tied very heavily to the existence of a server. In order to properly test the client, a mock would be required, which I don't know off hand how to do, though some googling would certainly get me there.

There isn't a Kubernetes deployment script, but given it's just an HTTP server grabbing a single port, it would also likely be trivially small.