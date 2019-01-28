# Sersan - Server Session for Golang

Package sersan implement traditional server session. Users who don't have a session
yet are assigned a random 32byte session ID and encoded using base32. All session
data is saved on a storage backend.

This package includes 2 implementation of *Backend (storage)*. It includes:

- Redis: Storage backend for using *Redis* via [redigo](https://github.com/gomodule/redigo).
- Recorder(testing): Storage backend for testing purpose.

The API is simple. Here an example that shows the sersan API:

```go
	import (
		"os"
		"net/http"
		"strconv"
		"github.com/syaiful6/sersan"
	)

	// Replace the storage variable with `Storage` implementation.
	var serversession = sersan.NewServerSessionState(storage, []byte(os.GetEnv("SECRET_KEY")))

	func MyHTTPHandler(w http.ResponseWriter, r *http.Request) {
		var count int = 0
		session, err := sersan.GetSession(r)
		// `sersan.GetSession` only return non nil error if you don't use our middleware.
		if err != nil {
			http.Error(w, "improperly configuration", http.StatusInternalServerError)
			return
		}

		// session is `map[interface{}]interface{}`
		if v, ok := session["count"]; ok {
			count = v.(int)
		}
		session["count"] = count + 1
		w.Write([]byte("You alredy visited this page " + strconv.Itoa(count) + " times"))

	}

	http.ListenAndServe(":8080", sersan.SessionMiddleware(serversession)(MyHTTPHandler))
```

First you need to get `Storage` implementation, and pass that to `NewServerSessionState`
with a secret key used to authenticate cookie value. Then you wrap your handler with `SessionMiddleware`.
Inside your handler you can modify/read/delete your session data.

## Authentication integration

This package have special support for authentication code or implementation.

- The authentication key used by authentication code (`_authID` by default) is saved
separately on storage backend. This allows you to quickly identify all sessions of a given user.
For example, you're able to implement a "log out everywhere" button.
- Whenever the logged in user changes, the backend will also invalidate the current session ID and
migrate the session data to a new ID. This prevents session fixation attacks while still
allowing you to maintain session state accross login/logout boundaries.