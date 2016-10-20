# tcpserver
A TCP Server with simple and clean API - for a sample see tests.

We just need a `struct` that satisfies `Processor` interface. The interesting part is our `struct` can be stateful, because each connection has it's own instance of our `struct`.

We implement it in a synchronous way because our `struct` is a closure inside the dedicated go-routine of the connection, so there is no need for extra go-routines.

If `Process` panics, it will get recovered and if it returns an `error`, connection will get closed and `Process` will no longer gets called.