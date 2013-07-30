package selfdestruct

import (
	"sync"
	"time"

	"github.com/badgerodon/collections/splay"
	. "github.com/nu7hatch/gouuid"
)

type (
	element struct {
		key     UUID
		message string
		expires time.Time
	}
	SelfDestructor struct {
		byKey  *splay.SplayTree
		byDate *splay.SplayTree
		lock   sync.Mutex
	}
)

var (
	AfterViewingExpiration = time.Second * 60
)

// compare two UUIDs
func less(a, b UUID) bool {
	for i := 0; i < 16; i++ {
		switch {
		case a[i] < b[i]:
			return true
		case a[i] > b[i]:
			return false
		}
	}
	return false
}

// Create a new SelfDestructor. Messages are cleaned once per second.
func New() *SelfDestructor {
	this := &SelfDestructor{
		byKey: splay.New(func(a, b interface{}) bool {
			return less(a.(element).key, b.(element).key)
		}),
		byDate: splay.New(func(a, b interface{}) bool {
			av := a.(element).expires
			bv := b.(element).expires
			// in case two keys have identical expire timestamps
			if av == bv {
				return less(a.(element).key, b.(element).key)
			}
			return bv.After(av)
		}),
	}
	go this.cleaner()
	return this
}

func (this *SelfDestructor) cleaner() {
	for {
		now := time.Now()

		this.lock.Lock()
		next := this.byDate.First()
		if next != nil && now.After(next.(element).expires) {
			this.byDate.Remove(next)
			this.byKey.Remove(next)
		}
		this.lock.Unlock()

		time.Sleep(time.Second)
	}
}

// Add a new message to the self destructor
func (this *SelfDestructor) Add(key UUID, message string, expires time.Time) bool {
	this.lock.Lock()
	defer this.lock.Unlock()

	el := element{
		key:     key,
		message: message,
		expires: expires,
	}

	if this.byKey.Has(el) {
		return false
	}

	this.byKey.Add(el)
	this.byDate.Add(el)

	return true
}

// Get a message from the self destructor.
func (this *SelfDestructor) Get(key UUID) (string, bool) {
	this.lock.Lock()
	defer this.lock.Unlock()

	el, ok := this.byKey.Get(element{key: key}).(element)
	if !ok {
		return "", false
	}

	el, ok = this.byDate.Get(element{key: key, expires: el.expires}).(element)
	if !ok {
		return "", false
	}

	newExpires := time.Now().Add(AfterViewingExpiration)
	if el.expires.After(newExpires) {
		el.expires = newExpires
		this.byKey.Add(el)
		this.byDate.Add(el)
	}

	return el.message, true
}
