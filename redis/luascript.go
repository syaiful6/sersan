package redis

import (
	"github.com/gomodule/redigo/redis"
)

var destroyScript = redis.NewScript(2, `
	redis.call('DEL', KEYS[1])
	-- delete auth
	if(KEYS[2] ~= '') then
		redis.call('SREM', KEYS[2], KEYS[1])
	end
	return true
`)

var destroyAllOfAuthIdScript = redis.NewScript(1, `
	local authIds = redis.call('SMEMBERS', KEYS[1])
	return redis.call('DEL', KEYS[1], unpack(authIds))
`)

var insertScript = redis.NewScript(2, `
	redis.call('SETEX', KEYS[1], ARGV[1], ARGV[2])
	if(KEYS[2] ~= '') then
		redis.call('SADD', KEYS[2], KEYS[1])
	end

	return true
`)

var replaceScript = redis.NewScript(3, `
	redis.call('DEL', KEYS[1])
	redis.call('SETEX', KEYS[1], ARGV[1], ARGV[2])
	if(KEYS[2] ~= KEYS[3]) then
		if(KEYS[2] ~= '') then
			redis.call('SREM', KEYS[2], KEYS[1])
		end
		if(KEYS[3] ~= '') then
			redis.call('SADD', KEYS[3], KEYS[1])
		end
	end

	return true
`)
