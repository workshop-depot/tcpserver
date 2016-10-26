# tcpserver
A TCP Server with simple and clean API - for a sample see tests.

We just need a `struct` that satisfies the `Agent` interface. The interesting part is that, our `struct` can be stateful, because each new connection, gets it's own instance of our `struct` - the aganet.

We implement it in a synchronous way because our `struct` is a closure inside the dedicated go-routine of the connection, so there is no need for extra go-routines.

If `Proceed` panics, it will get recovered and if it returns an `error`, connection will get closed and `Proceed` will no longer gets called. To stop the agent, you could simply return an error such as the already provided `ErrStop`.

This package provides the underlying server logic and strategies for client timeout or noticing inactive clients can get implemented inside the agents, based on the requirements of the problem at hand. No extra housekeeping tooling is needed because our agents manage all aspects of a client/connection.