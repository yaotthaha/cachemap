package cachemap

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"time"
)

type CacheMap = *cacheMapWrapper

type CallFuncType func(item CacheItem)

type CacheItem struct {
	Key        interface{}
	Value      interface{}
	TTL        time.Duration
	UpdateTime time.Time
	callFunc   CallFuncType
}

type cacheMap struct {
	m          map[interface{}]*CacheItem
	lock       sync.RWMutex
	stopChan   chan struct{}
	stopStatus bool
	sleepTime  time.Duration
}

type cacheMapWrapper struct {
	*cacheMap
}

type Option struct {
	SleepTime time.Duration
}

const (
	ErrorInvalidKeyType = "invalid key type"
	ErrorKeyNotFound    = "key not found"
	ErrorKeyExist       = "key exist"
)

type cacheMapInterface interface {
	Add(key, value interface{}, ttl time.Duration, callFunc CallFuncType) error
	Del(key interface{}) error
	Get(key interface{}) (CacheItem, error)
	SetValue(key, value interface{}) error
	SetTTL(key interface{}, ttl time.Duration, resetUpdateTime bool) error
	SetCallFunc(key interface{}, callFunc CallFuncType) error
	Foreach(fn func(item CacheItem))
	Stop()
}

func (cm *cacheMap) cacheRun() {
	for {
		select {
		case <-cm.stopChan:
			return
		case <-time.After(cm.sleepTime):
			cm.lock.Lock()
			for k, v := range cm.m {
				if v.TTL > 0 && v.UpdateTime.Add(v.TTL).Before(time.Now()) {
					if v.callFunc != nil {
						v.callFunc(*v)
					}
					delete(cm.m, k)
				}
			}
			cm.lock.Unlock()
		}
	}
}

func newCacheMap() *cacheMap {
	cm := &cacheMap{
		m:          make(map[interface{}]*CacheItem),
		lock:       sync.RWMutex{},
		stopChan:   make(chan struct{}),
		stopStatus: false,
		sleepTime:  800 * time.Millisecond,
	}
	return cm
}

//停止运行
func (w *cacheMapWrapper) Stop() {
	w.stopChan <- struct{}{}
	w.stopStatus = true
	close(w.stopChan)
}

// 创建一个 Cache Map
func NewCacheMap(options ...Option) CacheMap {
	w := &cacheMapWrapper{newCacheMap()}
	if len(options) > 0 {
		for _, v := range options {
			if v.SleepTime > 0 {
				w.sleepTime = v.SleepTime
			}
		}
	}
	go w.cacheRun()
	runtime.SetFinalizer(w, (*cacheMapWrapper).Stop)
	return w
}

func CheckKeyType(key interface{}) (string, bool) {
	Kind := reflect.ValueOf(key).Kind()
	switch {
	case Kind == reflect.Map:
		return Kind.String(), false
	case Kind == reflect.Slice:
		return Kind.String(), false
	case Kind == reflect.Func:
		return Kind.String(), false
	}
	return Kind.String(), true
}

func (cm *cacheMap) add(key, value interface{}, ttl time.Duration, callFunc CallFuncType) error {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	if tp, ok := CheckKeyType(key); !ok {
		return errors.New(fmt.Sprintf(ErrorInvalidKeyType+": %s", tp))
	}
	_, ok := cm.m[key]
	if !ok {
		item := &CacheItem{
			Key:        key,
			Value:      value,
			TTL:        ttl,
			UpdateTime: time.Now(),
			callFunc:   callFunc,
		}
		cm.m[key] = item
		return nil
	} else {
		return errors.New(ErrorKeyExist)
	}
}

// 添加一个键值对
func (w *cacheMapWrapper) Add(key, value interface{}, ttl time.Duration, callFunc CallFuncType) error {
	return w.add(key, value, ttl, callFunc)
}

func (cm *cacheMap) del(key interface{}) error {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	if tp, ok := CheckKeyType(key); !ok {
		return errors.New(fmt.Sprintf(ErrorInvalidKeyType+": %s", tp))
	}
	item, ok := cm.m[key]
	if ok {
		if item.TTL > 0 && item.UpdateTime.Add(item.TTL).Before(time.Now()) {
			delete(cm.m, key)
			return errors.New(ErrorKeyNotFound)
		} else {
			delete(cm.m, key)
			return nil
		}
	} else {
		return errors.New(ErrorKeyNotFound)
	}
}

// 删除一个键值对
func (w *cacheMapWrapper) Del(key interface{}) error {
	return w.del(key)
}

func (cm *cacheMap) get(key interface{}) (CacheItem, error) {
	cm.lock.RLock()
	defer cm.lock.RUnlock()
	if tp, ok := CheckKeyType(key); !ok {
		return CacheItem{}, errors.New(fmt.Sprintf(ErrorInvalidKeyType+": %s", tp))
	}
	item, ok := cm.m[key]
	if ok {
		return *item, nil
	} else {
		return CacheItem{}, errors.New(ErrorKeyNotFound)
	}
}

// 获取一个键值对信息
func (w *cacheMapWrapper) Get(key interface{}) (CacheItem, error) {
	return w.get(key)
}

func (cm *cacheMap) setValue(key, value interface{}) error {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	if tp, ok := CheckKeyType(key); !ok {
		return errors.New(fmt.Sprintf(ErrorInvalidKeyType+": %s", tp))
	}
	item, ok := cm.m[key]
	if ok {
		item.Value = value
		return nil
	} else {
		return errors.New(ErrorKeyNotFound)
	}
}

// 设置值
func (w *cacheMapWrapper) SetValue(key, value interface{}) error {
	return w.setValue(key, value)
}

func (cm *cacheMap) setTTL(key interface{}, ttl time.Duration, resetUpdateTime bool) error {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	if tp, ok := CheckKeyType(key); !ok {
		return errors.New(fmt.Sprintf(ErrorInvalidKeyType+": %s", tp))
	}
	item, ok := cm.m[key]
	if ok {
		item.TTL = ttl
		if resetUpdateTime {
			item.UpdateTime = time.Now()
		}
		return nil
	} else {
		return errors.New(ErrorKeyNotFound)
	}
}

//设置TTL
func (w *cacheMapWrapper) SetTTL(key interface{}, ttl time.Duration, resetUpdateTime bool) error {
	return w.setTTL(key, ttl, resetUpdateTime)
}

func (cm *cacheMap) setCallFunc(key interface{}, callFunc CallFuncType) error {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	if tp, ok := CheckKeyType(key); !ok {
		return errors.New(fmt.Sprintf(ErrorInvalidKeyType+": %s", tp))
	}
	item, ok := cm.m[key]
	if ok {
		item.callFunc = callFunc
		return nil
	} else {
		return errors.New(ErrorKeyNotFound)
	}
}

//设置唤醒函数
func (w *cacheMapWrapper) SetCallFunc(key interface{}, callFunc CallFuncType) error {
	return w.setCallFunc(key, callFunc)
}

func (cm *cacheMap) foreach(fn CallFuncType) {
	cm.lock.RLock()
	defer cm.lock.RUnlock()
	for _, v := range cm.m {
		fn(*v)
	}
}

//遍历 Map
func (w *cacheMapWrapper) Foreach(fn CallFuncType) {
	w.foreach(fn)
}

// 清除所有键值对
func (w *cacheMapWrapper) Clear() {
	w.clear()
}

func (cm *cacheMap) clear() {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	cm.m = make(map[interface{}]*CacheItem)
}
