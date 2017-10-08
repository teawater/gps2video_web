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

type User struct {
	Token string
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

			output_dir := filepath.Join(users.dir, fmt.Sprintf("%d", uid), "output")
			exist, err := fileIsExist(output_dir)
			if err != nil {
				log.Println(uid, "Init fileIsExist:", output_dir, err)
				os.RemoveAll(path)
				return nil
			}
			if exist {
				go makeVideo(output_dir)
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

func (u *UserMap) Find_add(token string) (uid uint64, err error) {
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
		user_dir := filepath.Join(u.dir, fmt.Sprintf("%d", uid))

		if err = os.RemoveAll(user_dir); err != nil {
			return
		}
		if err = os.Mkdir(user_dir, os.FileMode(0700)); err != nil {
			return
		}
		var fd *os.File
		if fd, err = os.Create(filepath.Join(user_dir, "user.gob")); err != nil {
			return
		}
		defer fd.Close()
		enc := gob.NewEncoder(fd)

		//Get user
		user := new(User)
		user.Token = token

		if err = enc.Encode(user); err != nil {
			os.RemoveAll(user_dir)
			return
		}

		u.token2uid[token] = uid
		u.uid2user[uid] = user
		u.last_uid = uid
	}
	return
}
