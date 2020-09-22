// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package stats provides a tool to collect and track statistics
// Example usage:
// 	c := NewStatCollector("myapp")
// 	c.Child("performance").Child("net-latency").Value(200)
// 	c.Ch("performance").Ch("iops").V(100)
// 	c.Ch("hits") //defaults to int(0)
// 	c.Ch("hits").Incr() // add 1 hit
// 	fmt.Println(c.JSON())
// Outputs
// 	{"init": "2015-10-05T13:01:42.722509063-04:00",
// 	"changed": "0001-01-01T00:00:00Z",
// 	"value": 0,
// 	"children": {
// 		"hits": {
// 			"init": "2015-10-05T13:01:42.722511672-04:00",
// 			"changed": "0001-01-01T00:00:00Z",
// 			"value": 1
// 		},
// 		"performance": {
// 			"init": "2015-10-05T13:01:42.72250972-04:00",
// 			"changed": "0001-01-01T00:00:00Z",
// 			"value": 0,
// 			"children": {
// 				"iops": {
// 					"init": "2015-10-05T13:01:42.722511208-04:00",
// 					"changed": "2015-10-05T13:01:42.722511477-04:00",
// 					"value": 100
// 				},
// 				"net-latency": {
// 					"init": "2015-10-05T13:01:42.7225103-04:00",
// 					"changed": "2015-10-05T13:01:42.722510848-04:00",
// 					"value": 200
// 				}
// 			}
// 		}
// 	}}
package stats

import (
	"encoding/json"
	"sync"
	"time"
)

// Collector stores a stat name, value, and its children. DisableTime is inherited by children automatically
type Collector struct {
	StatName    string                `json:"-"`
	InitDate    *time.Time            `json:"init,omitempty"`
	ChangeDate  *time.Time            `json:"changed,omitempty"`
	StatValue   interface{}           `json:"value"`
	Children    map[string]*Collector `json:"children,omitempty"`
	DisableTime bool                  `json:"-"`

	mutex sync.RWMutex
}

// NewStatCollector Creates an empty new collector with default stat value of int64(0)
// Enables timestamp collection by default
func NewStatCollector(name string) *Collector {
	c := Collector{}
	c.StatName = name
	c.StatValue = int64(0)
	c.InitDate = nowPointer()
	c.DisableTime = false
	return &c
}

// JSON Returns the collector and its children in serialized JSON format
// 		{
//			"init": "1999-10-05T12:25:08.793401767-04:00",
//			"changed": "0001-01-01T00:00:00Z",
//			"value": 0,
//			"children": {
//				"c1": {
//					"init": "1999-10-05T12:25:08.793402159-04:00",
//					"changed": "2001-10-05T12:25:08.79340264-04:00",
//					"value": 1
//				}
//			}
// 		}
func (c *Collector) JSON() (string, error) {
	c.deepLock()
	defer c.deepUnlock()
	stats, err := json.MarshalIndent(c, "", "\t")
	return string(stats), err
}

func (c *Collector) deepLock() {
	c.mutex.Lock()
	for _, ch := range c.Children {
		ch.deepLock()
	}
}

func (c *Collector) deepUnlock() {
	c.mutex.Unlock()
	for _, ch := range c.Children {
		ch.deepUnlock()
	}
}

// DisableTimes sets init to 0, changed to 0, and disables the timestamp collection for this item
func (c *Collector) DisableTimes() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.DisableTime = true
	c.InitDate = nil
	c.ChangeDate = nil
}

// EnableTimes sets init to time.Now(), leaves changed alone, and enables the timestamp collection for this item
func (c *Collector) EnableTimes() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.DisableTime = false
	c.InitDate = nowPointer()
}

// Child retrives (and creates, if does not exist) a child stat by name
func (c *Collector) Child(name string) *Collector {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.Children == nil {
		c.Children = make(map[string]*Collector)
	}

	child, ok := c.Children[name]
	if !ok {
		child = NewStatCollector(name)
		if c.DisableTime {
			child.DisableTimes()
		}
		c.Children[name] = child
	}
	return child
}

// Ch is the short function alias for Child()
func (c *Collector) Ch(name string) *Collector {
	return c.Child(name)
}

// Has checks if a child stat name exists on the collector, returns true if exists
func (c *Collector) Has(name string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.Children == nil {
		return false
	}

	_, ok := c.Children[name]
	return ok
}

// Name returns the collectors stat name
func (c *Collector) Name() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.StatName
}

// GetV returns the collectors stat value
func (c *Collector) GetV() interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.StatValue
}

// Value sets the collectors stat value
func (c *Collector) Value(v interface{}) *Collector {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.StatValue = v
	if !c.DisableTime {
		c.ChangeDate = nowPointer()
	}
	return c
}

// V is the short function alias for Value()
func (c *Collector) V(v interface{}) *Collector {
	return c.Value(v)
}

// Vi is the short function alias for Value() which accepts only an int64
func (c *Collector) Vi(i int64) *Collector {
	return c.V(i)
}

// Vf is the short function alias for Value() which accepts only a float64
func (c *Collector) Vf(f float64) *Collector {
	return c.V(f)
}

// Vs is the short function alias for Value() which accepts only a string
func (c *Collector) Vs(s string) *Collector {
	return c.V(s)
}

// Incr adds to the current stat value by 1 (or 1.0 for float)
// Supported types: int, int32, int64, float32, float64, others are ignored
func (c *Collector) Incr() *Collector {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	switch c.StatValue.(type) {
	case int:
		c.StatValue = c.StatValue.(int) + 1
	case int32:
		c.StatValue = c.StatValue.(int32) + 1
	case int64:
		c.StatValue = c.StatValue.(int64) + 1
	case float32:
		c.StatValue = c.StatValue.(float32) + 1.0
	case float64:
		c.StatValue = c.StatValue.(float64) + 1.0
	}
	if !c.DisableTime {
		c.ChangeDate = nowPointer()
	}
	return c
}

// Decr subtracts from the current stat value by 1 (or 1.0 for float)
// Supported types: int, int32, int64, float32, float64, others are ignored
func (c *Collector) Decr() *Collector {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	switch c.StatValue.(type) {
	case int:
		c.StatValue = c.StatValue.(int) - 1
	case int32:
		c.StatValue = c.StatValue.(int32) - 1
	case int64:
		c.StatValue = c.StatValue.(int64) - 1
	case float32:
		c.StatValue = c.StatValue.(float32) - 1.0
	case float64:
		c.StatValue = c.StatValue.(float64) - 1.0
	}
	if !c.DisableTime {
		c.ChangeDate = nowPointer()
	}
	return c
}

// AvgLen sets the value as an array where index 0 is the computed average.
// It sets the length of items to store to compute that average, if it has not been set.
// If it has been set already, then it will ignore this call.
func (c *Collector) AvgLen(init float32, length int) *Collector {
	c.mutex.RLock()

	switch c.StatValue.(type) {
	case []float32:
		//do nothing for now except add the value to the avg. perhaps in future, resize slice
		c.mutex.RUnlock()
	default:
		c.mutex.RUnlock()
		v := []float32{}
		for i := 0; i < length+1; i++ {
			v = append(v, init)
		}
		c.V(v)
	}
	return c
}

// Avg takes the next value in a sequence to average and store for this stat
// If AvgLen hasn't been called yet to init this stat, default length of 10 is used.
func (c *Collector) Avg(nextval float32) *Collector {
	//try to init the avg. if already initialized, this call wont do anything
	c.AvgLen(nextval, 10)

	switch c.StatValue.(type) {
	case []float32:
		a := c.StatValue.([]float32)
		a[0] = nextval
		l := len(a) - 1
		var t float32
		for i := 0; i < l; i++ {
			t += a[i]
		}
		avg := t / float32(l)
		var newst = []float32{avg}
		c.V(append(newst, a[0:l]...))
	}
	return c
}

// nowPointer gets Time.Now() in pointer format for use with omitempty marshaling
func nowPointer() *time.Time {
	t := time.Now()
	return &t
}
