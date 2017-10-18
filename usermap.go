package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

const (
	UserNormal = iota
	UserMakingVideo
	UserMakeVideoFail
)

type User struct {
	Token  string
	Status int

	Moptions            MakeVideoOptions
	MakeVideoFailReason string
}

type UserMap struct {
	lock sync.RWMutex

	uid2user map[uint64]*User

	token2uid map[string]uint64

	last_uid uint64

	dir string
}

var users UserMap

func (u *UserMap) Init(dir string) {
	u.dir = dir

	u.uid2user = make(map[uint64]*User)
	u.token2uid = make(map[string]uint64)

	err := dir_check_creat(u.dir, false)
	if err != nil {
		log.Fatal(err)
	}

	err = filepath.Walk(u.dir, func(path string, f os.FileInfo, err error) error {
		if f == nil {
			return err
		}
		if !f.IsDir() || path == u.dir {
			return nil
		}

		match, err := filepath.Match(`[0-9]*`, f.Name())
		if err == nil && match {
			var fd *os.File
			if fd, err = os.Open(filepath.Join(path, "user.gob")); err != nil {
				os.RemoveAll(path)
				return nil
			}
			dec := gob.NewDecoder(fd)
			user := new(User)
			if dec.Decode(user) != nil {
				os.RemoveAll(path)
				return nil
			}
			fd.Close()
			var uid uint64
			if uid, err = strconv.ParseUint(f.Name(), 10, 64); err != nil {
				os.RemoveAll(path)
				return nil
			}
			u.uid2user[uid] = user
			u.token2uid[user.Token] = uid

			if user.Status == UserMakingVideo {
				log.Println("ReMakingVideo", uid)
				go makeVideo(uid, user.Token, &user.Moptions)
			}

			log.Println("Add ", uid, user.Token)
		}
		return filepath.SkipDir
	})
	if err != nil {
		log.Fatal(err)
	}
}

func (u *UserMap) Check(uid uint64, token string) bool {
	u.lock.RLock()
	defer u.lock.RUnlock()

	got_uid, ok := u.token2uid[token]
	if (!ok) || (got_uid != uid) {
		return false
	}
	return true
}

//Must hold u.lock.Lock
func (u *UserMap) Write(userDir string, user *User) error {
	fd, err := os.Create(filepath.Join(userDir, "user.gob"))
	if err != nil {
		return err
	}
	defer fd.Close()
	enc := gob.NewEncoder(fd)
	err = enc.Encode(user)
	return nil
}

func (u *UserMap) FindAdd(token string) (uid uint64, err error) {
	u.lock.Lock()
	defer u.lock.Unlock()

	uid, ok := u.token2uid[token]
	if !ok {
		//Get uid
		uid = u.last_uid
		for {
			uid++
			_, ok := u.uid2user[uid]
			if !ok {
				break
			}
		}
		userDir := filepath.Join(u.dir, fmt.Sprintf("%d", uid))

		//Create userDir
		if err = os.RemoveAll(userDir); err != nil {
			return
		}
		if err = os.Mkdir(userDir, os.FileMode(0700)); err != nil {
			return
		}

		//Get user
		user := new(User)
		user.Token = token
		user.Status = UserNormal

		if err = u.Write(userDir, user); err != nil {
			os.RemoveAll(userDir)
			return
		}

		u.token2uid[token] = uid
		u.uid2user[uid] = user
		u.last_uid = uid
	}
	return
}

func (u *UserMap) GetUserStatus(uid uint64) (status int, err error) {
	u.lock.RLock()
	defer u.lock.RUnlock()

	user, ok := u.uid2user[uid]
	if !ok {
		err = fmt.Errorf("查找客户%d失败", uid)
		return
	}

	status = user.Status
	return
}

func (u *UserMap) SetUserStatus(uid uint64, status int, options *MakeVideoOptions, reason string) (err error) {
	u.lock.Lock()
	defer u.lock.Unlock()

	user, ok := u.uid2user[uid]
	if !ok {
		err = fmt.Errorf("查找客户%d失败", uid)
		return
	}
	if status < UserNormal || status > UserMakeVideoFail {
		err = fmt.Errorf("设置客户%d状态%d失败", uid, status)
		return
	}

	old_status := user.Status
	user.Status = status
	user.MakeVideoFailReason = reason

	if options != nil {
		user.Moptions = *options
	}

	userDir := filepath.Join(u.dir, fmt.Sprintf("%d", uid))
	if err = u.Write(userDir, user); err != nil {
		user.Status = old_status
		return
	}

	return
}

func (u *UserMap) GetUserMakeVideoFailReason(uid uint64) (reason string) {
	u.lock.RLock()
	defer u.lock.RUnlock()

	user, ok := u.uid2user[uid]
	if !ok {
		reason = ""
		return
	}

	reason = user.MakeVideoFailReason
	return
}
