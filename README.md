# rtunnel

HTTP tunneling server/client.

# Usage

First, spin up some tunnels with `rtunneld` on your server.

```console
example.com$ rtunneld 8080 8081 8082
```

Then, from your laptop, claim one of the tunnels with `rtunnel` to make your laptop the exit for the tunnel.

```console
laptop$ rtunnel http://example.com:8081
```

Now you can access any websites with your laptop's IP address.

```console
example.com$ curl -p -x http://example.com:8081 https://httpbin.org/ip
{
  "origin": "[YOUR LAPTOP'S IP ADDRESS]"
}
```
