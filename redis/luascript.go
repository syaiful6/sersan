package redis

import (
	"github.com/gomodule/redigo/redis"
)

// Lua script for destroying session
//
// KEYS[1] - the session's ID to be removed
// ARGS[1] - prefix for AuthID
var destroyScript = redis.NewScript(1, `
	local authID = redis.call('HGET', KEYS[1], 'AuthID')
	redis.call('DEL', KEYS[1])
	-- delete auth
	if(authID ~= nil) then
		redis.call('SREM', ARGV[1] .. authID, KEYS[1])
	end
	return true
`)

// Lua script for destroying all sessions for provided authID
//
// KEYS[1] - AuthID
var destroyAllOfAuthIdScript = redis.NewScript(1, `
	local sessions = redis.call('SMEMBERS', KEYS[1])
	return redis.call('DEL', KEYS[1], unpack(sessions))
`)

// Lua script for inserting session
//
// KEYS[1] - session's ID
// KEYS[2] - Auth key
// ARGV[1] - Expiration in seconds
// ARGV... - Session Data
var insertScript = redis.NewScript(2, `
	local ex = redis.call('EXISTS', KEYS[1])
	if(ex > 0) then
		return redis.error_reply('Session already exists')
	end

	-- now insert session data
	local sessions = {}
	for i = 2, #ARGV, 1 do
		sessions[#sessions + 1] = ARGV[i]
	end
	redis.call('HMSET', KEYS[1], unpack(sessions))

	-- expire if needed
	if(ARGV[1] ~= 0) then
		redis.call('EXPIRE', KEYS[1], ARGV[1])
	end

	if(KEYS[2] ~= '') then
		redis.call('SADD', KEYS[2], KEYS[1])
	end

	return redis.status_reply('OK')
`)

// Lua script for replace/update session
//
// KEYS[1] - Session ID
// KEYS[2] - Current auth key
// ARGV[1] - expiration in second
// ARGV[2] - Auth prefix
var replaceScript = redis.NewScript(2, `
	local ex = redis.call('EXISTS', KEYS[1])
	if(ex == 0) then
		return redis.error_reply('Session didnt exists')
	end
	local oldAuthID = redis.call('HGET', KEYS[1], 'AuthID')
	if(oldAuthID ~= nil) then
		oldAuthID = ARGV[2] .. oldAuthID
	else
		oldAuthID = ''
	end
	redis.call('DEL', KEYS[1])
	redis.call('HMSET', KEYS[1], unpack(ARGV))
	redis.call('EXPIRE', KEYS[1], ARGV[1])
	if(KEYS[2] ~= oldAuthID) then
		if(oldAuthID ~= '') then
			redis.call('SREM', oldAuthID, KEYS[1])
		end
		if(KEYS[2] ~= '') then
			redis.call('SADD', KEYS[2], KEYS[1])
		end
	end

	return redis.status_reply('OK')
`)
