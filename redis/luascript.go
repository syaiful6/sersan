package redis

import (
	"github.com/gomodule/redigo/redis"
)

// Lua script for inserting session
//
// KEYS[1] - session's ID
// KEYS[2] - Auth key
// ARGV[1] - Expiration in seconds
// ARGV... - Session Data
var insertScript = redis.NewScript(2, `
	-- now insert session data
	local sessions = {}
	for i = 2, #ARGV, 1 do
		sessions[#sessions + 1] = ARGV[i]
	end
	redis.call('HMSET', KEYS[1], unpack(sessions))

	-- expire if needed
	if(ARGV[1] ~= '') then
		redis.call('EXPIRE', KEYS[1], ARGV[1])
	end

	if(KEYS[2] ~= '') then
		redis.call('SADD', KEYS[2], KEYS[1])
	end

	return true
`)

// Lua script for replace/update session
//
// KEYS[1] - Session ID
// KEYS[2] - Current auth key
// KEYS[3] - Old auth key
// ARGV[1] - expiration in second
// ARGV... - session data
var replaceScript = redis.NewScript(3, `
	redis.call('DEL', KEYS[1])
	local sessions = {}
	for i = 2, #ARGV, 1 do
		sessions[#sessions + 1] = ARGV[i]
	end
	redis.call('HMSET', KEYS[1], unpack(sessions))
	-- expire if needed
	if(ARGV[1] ~= '') then
		redis.call('EXPIRE', KEYS[1], ARGV[1])
	end
	-- if old authID is not equal with new one, replace that
	if(KEYS[2] ~= KEYS[3]) then
		if(KEYS[3] ~= '') then
			redis.call('SREM', KEYS[3], KEYS[1])
		end
		if(KEYS[2] ~= '') then
			redis.call('SADD', KEYS[2], KEYS[1])
		end
	end

	return true
`)
