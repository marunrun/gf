// Copyright 2018 gf Author(https://gitee.com/johng/gf). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://gitee.com/johng/gf.

package gcron

import (
    "errors"
    "fmt"
    "gitee.com/johng/gf/g/container/garray"
    "gitee.com/johng/gf/g/container/gmap"
    "gitee.com/johng/gf/g/container/gtype"
    "gitee.com/johng/gf/g/os/gtimer"
    "strconv"
    "time"
)

// 定时任务管理对象
type Cron struct {
    idgen    *gtype.Int               // 用于唯一名称生成
    status   *gtype.Int               // 定时任务状态(0: 未执行; 1: 运行中; 2: 已停止; -1:删除关闭)
    entries  *gmap.StringInterfaceMap // 所有的定时任务项
}

// 创建自定义的定时任务管理对象
func New() *Cron {
    cron := &Cron {
        idgen    : gtype.NewInt(1000000),
        status   : gtype.NewInt(STATUS_RUNNING),
        entries  : gmap.NewStringInterfaceMap(),
    }
    gtimer.Add(time.Second, func() {
        if cron.status.Val() == STATUS_CLOSED {
            gtimer.Exit()
        }
        if cron.status.Val() == STATUS_RUNNING {
            go cron.checkEntries(time.Now())
        }
    })
    return cron
}

// 添加定时任务
func (c *Cron) Add(pattern string, job func(), name ... string) (*Entry, error) {
    if len(name) > 0 {
        if c.Search(name[0]) != nil {
            return nil, errors.New(fmt.Sprintf(`cron job "%s" already exists`, name[0]))
        }
    }
    entry, err := newEntry(pattern, job, false, gDEFAULT_TIMES, name ...)
    if err != nil {
        return nil, err
    }
    if len(name) > 0 {
        entry.Name = name[0]
    } else {
        entry.Name = strconv.Itoa(c.idgen.Add(1))
    }
    c.entries.Set(entry.Name, entry)
    return entry, nil
}

// 添加单例运行定时任务
func (c *Cron) AddSingleton(pattern string, job func(), name ... string) (*Entry, error) {
    if entry, err := c.Add(pattern, job, name ...); err != nil {
        return nil, err
    } else {
        entry.SetSingleton(true)
        return entry, nil
    }
}

// 添加只运行一次的定时任务
func (c *Cron) AddOnce(pattern string, job func(), name ... string) (*Entry, error) {
    if entry, err := c.Add(pattern, job, name ...); err != nil {
        return nil, err
    } else {
        entry.SetTimes(1)
        return entry, nil
    }
}

// 添加运行指定次数的定时任务
func (c *Cron) AddTimes(pattern string, times int, job func(), name ... string) (*Entry, error) {
    if entry, err := c.Add(pattern, job, name ...); err != nil {
        return nil, err
    } else {
        entry.SetTimes(times)
        return entry, nil
    }
}

// 延迟添加定时任务，delay参数单位为秒
func (c *Cron) DelayAdd(delay int, pattern string, job func(), name ... string) {
    gtimer.AddOnce(time.Duration(delay)*time.Second, func() {
        if _, err := c.Add(pattern, job, name ...); err != nil {
            panic(err)
        }
    })
}

// 延迟添加单例定时任务，delay参数单位为秒
func (c *Cron) DelayAddSingleton(delay int, pattern string, job func(), name ... string) {
    gtimer.AddOnce(time.Duration(delay)*time.Second, func() {
        if _, err := c.AddSingleton(pattern, job, name ...); err != nil {
            panic(err)
        }
    })
}

// 延迟添加运行指定次数的定时任务，delay参数单位为秒
func (c *Cron) DelayAddOnce(delay int, pattern string, job func(), name ... string) {
    gtimer.AddOnce(time.Duration(delay)*time.Second, func() {
        if _, err := c.AddOnce(pattern, job, name ...); err != nil {
            panic(err)
        }
    })
}

// 延迟添加只运行一次的定时任务，delay参数单位为秒
func (c *Cron) DelayAddTimes(delay int, pattern string, times int, job func(), name ... string) {
    gtimer.AddOnce(time.Duration(delay)*time.Second, func() {
        if _, err := c.AddTimes(pattern, times, job, name ...); err != nil {
            panic(err)
        }
    })
}

// 检索指定名称的定时任务
func (c *Cron) Search(name string) *Entry {
    if v := c.entries.Get(name); v != nil {
        return v.(*Entry)
    }
    return nil
}

// 根据指定名称删除定时任务
func (c *Cron) Remove(name string) {
    c.entries.Remove(name)
}

// 开启定时任务执行(可以指定特定名称的一个或若干个定时任务)
func (c *Cron) Start(name...string) {
    if len(name) > 0 {
        for _, v := range name {
            if entry := c.Search(v); entry != nil {
                entry.Start()
            }
        }
    } else {
        c.status.Set(STATUS_READY)
    }
}

// 停止定时任务执行(可以指定特定名称的一个或若干个定时任务)
func (c *Cron) Stop(name...string) {
    if len(name) > 0 {
        for _, v := range name {
            if entry := c.Search(v); entry != nil {
                entry.Stop()
            }
        }
    } else {
        c.status.Set(STATUS_STOPPED)
    }
}

// 关闭定时任务
func (c *Cron) Close() {
    c.status.Set(STATUS_CLOSED)
}

// 获取所有已注册的定时任务数量
func (c *Cron) Size() int {
    return c.entries.Size()
}

// 获取所有已注册的定时任务项(按照注册时间从小到大进行排序)
func (c *Cron) Entries() []*Entry {
    array := garray.NewSortedArray(c.entries.Size(), func(v1, v2 interface{}) int {
        entry1 := v1.(*Entry)
        entry2 := v2.(*Entry)
        if entry1.Time.Nanosecond() > entry2.Time.Nanosecond() {
            return 1
        }
        return -1
    }, true)
    c.entries.RLockFunc(func(m map[string]interface{}) {
        for _, v := range m {
            array.Add(v.(*Entry))
        }
    })
    entries := make([]*Entry, array.Len())
    array.RLockFunc(func(array []interface{}) {
        for k, v := range array {
            entries[k] = v.(*Entry)
        }
    })
    return entries
}

// 遍历检查可执行定时任务，并异步执行
func (c *Cron) checkEntries(t time.Time) {
    removeKeys := make([]string, 0)
    c.entries.RLockFunc(func(m map[string]interface{}) {
        for k, v := range m {
            entry := v.(*Entry)
            if entry.schedule.meet(t) {
                // 是否停止
                if entry.status.Val() == STATUS_STOPPED {
                    continue
                }
                // 是否关闭
                if entry.status.Val() == STATUS_CLOSED {
                    removeKeys = append(removeKeys, k)
                    continue
                }
                // 单例模式
                if entry.status.Set(STATUS_RUNNING) == STATUS_RUNNING {
                    if entry.IsSingleton() {
                        continue
                    }
                }
                // 运行次数
                if t := entry.times.Add(-1); t <= 0 {
                    if entry.status.Set(STATUS_CLOSED) == STATUS_CLOSED {
                        continue
                    }
                } else if t < 2000000000 && t > 1000000000 {
                    entry.times.Set(gDEFAULT_TIMES)
                }
                // 执行异步运行
                go func() {
                    defer func() {
                        switch entry.status.Val() {
                            case STATUS_CLOSED:
                                c.Remove(entry.Name)

                            case STATUS_STOPPED:

                            default:
                                entry.status.Set(STATUS_READY)
                        }
                    }()
                    entry.Job()
                }()
            }
        }
    })
    if len(removeKeys) > 0 {
        c.entries.BatchRemove(removeKeys)
    }
}
