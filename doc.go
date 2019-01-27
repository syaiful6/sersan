/*
Package sersan implement traditional server session. Users who don't have a session
yet are assigned a random 32byte session ID and encoded using base32. All session
data is saved on a storage backend.

This package 2 implementation of *Backend (storage)*. It includes:

* Redis: Storage backend for using *Redis* via [https://github.com/gomodule/redigo](redigo).
* Recorder(testing): Storage backend for testing purpose.
*/
