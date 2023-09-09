package jobs

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/mylxsw/glacier/scheduler"
	"github.com/redis/go-redis/v9"
)

type LockManager struct {
	client      *redis.Client
	name        string
	value       string
	lockTimeout time.Duration
	lock        sync.Mutex
}

func New(client *redis.Client, name string, lockTimeout time.Duration) scheduler.LockManager {
	return &LockManager{client: client, name: name, lockTimeout: lockTimeout}
}

func (m *LockManager) TryLock(ctx context.Context) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.value == "" {
		var err error
		m.value, err = randomToken()
		if err != nil {
			return fmt.Errorf("generate lock token failed: %w", err)
		}
	}

	ret, err := lockScript.Run(ctx, m.client, []string{m.name, m.value}, m.lockTimeout.Milliseconds()).Result()
	if err != nil {
		return fmt.Errorf("try lock failed: %w", err)
	}

	if ret.(int64) == 0 {
		return scheduler.ErrLockFailed
	}

	return nil
}

var lockScript = redis.NewScript(`if redis.call('setnx', KEYS[1], KEYS[2]) == 1 then
	redis.call('pexpire', KEYS[1], ARGV[1])
	return 1
else
	local original = redis.call('get', KEYS[1])
	if KEYS[2] == original then
		redis.call('pexpire', KEYS[1], ARGV[1])
		return 1
	end
	return 0
end`)
var releaseScript = redis.NewScript(`if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`)

func (m *LockManager) Release(ctx context.Context) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	_, err := releaseScript.Run(ctx, m.client, []string{m.name}, m.value).Result()
	if err != nil {
		return fmt.Errorf("release lock failed: %w", err)
	}

	return nil
}

func randomToken() (string, error) {
	tmp := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, tmp); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(tmp), nil
}
