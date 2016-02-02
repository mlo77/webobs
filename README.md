About
=====

Web observer is a go library that creates websocket handlers at runtime for communicating between your go program and web browsers that support websockets. You could use it for monitoring or stats building.
I created this package so that I could debug and visualize results from my Go program on a browser with the ease of javascript/css/... 

Installation
============

  go get github.com/mlo77/webobs

Usage
=====

In your Go program, 
```
	// creates a http server on 8080 
	wobs := webobs.StartServer(":8080")

	// set a channel ready to be connected
	// from the address http://localhost:8080/fooTag
    wobs.SetChannel("fooTag", browserToThisAppCallback, "./pathToHtmlResources")

```
Open a browser and open `http://localhost:8080/fooTag`
The Go web server will serve /pathToHtmlResources/fooTag.html if it exists, otherwise it will generate a template that you can build up from.

The javascript side code typically looks like this
```
    var soc = new WebSocket("ws://"+window.location.host+"/"+{{.Tagws}})

    // receive data from Go app
    soc.onmessage = function (event) {
    	var obj = JSON.parse(event.data)
    	console.log(obj)
    }

    // send data to Go app
    soc.send(JSON.stringify({ foo:1, bar:0 }))

```

As soon as the browser hits the address, a websocket connection is setup allowing to pass data between Go and the browser.

Back to your Go program
```
	func browserToThisAppCallback(tag string, data []byte) {
	    var obj SomeStruct
	    err := json.Unmarshal(data, &obj)
	    if err != nil {
	        fmt.Println("error:", err)
	    }
	    ....
	}

	// Sends data to all browser connected to http://localhost:8080/fooTag
	wobs.WriteCh <- webobs.Message{ Tag: "fooTag", Data: []byte("hello") }

	// If there is no websocket connected the above line does nothing
```

Licence
=======

MIT Licence