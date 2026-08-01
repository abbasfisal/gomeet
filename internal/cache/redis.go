package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.Client
}

func New(addr, password string, db int) *Cache {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &Cache{client: client}
}

func (c *Cache) Close() error {
	return c.client.Close()
}

func (c *Cache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *Cache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, ttl).Err()
}

func (c *Cache) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (c *Cache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

func (c *Cache) DeleteByPattern(ctx context.Context, pattern string) error {
	iter := c.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := c.client.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

func RoomListKey(page, pageSize int) string {
	return fmt.Sprintf("rooms:list:%d:%d", page, pageSize)
}

func RoomKey(id uint) string {
	return fmt.Sprintf("room:%d", id)
}

func RoomAvailabilityKey(roomID uint, from, to string) string {
	return fmt.Sprintf("room:%d:availability:%s:%s", roomID, from, to)
}

// RoomAvailabilityPattern matches every cached availability key of a room, used
// for targeted cache invalidation after a booking is created or cancelled.
func RoomAvailabilityPattern(roomID uint) string {
	return fmt.Sprintf("room:%d:availability:*:*", roomID)
}

const (
	TTLRoomList      = 5 * time.Minute
	TTLRoomDetail    = 5 * time.Minute
	TTLRoomAvail     = 3 * time.Minute
	CachePrefixRooms = "rooms:*"
	CachePrefixRoom  = "room:*"
)
