package main

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"
	"tokenshare"

	"github.com/boltdb/bolt"
)

type database struct {
	db      *bolt.DB
	bucket  string
	tokSize int
}

func (d *database) init() error {
	return d.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(d.bucket))
		return err
	})
}

func (d *database) new() (tokenshare.Token, error) {
	buf := make([]byte, d.tokSize)
	if _, err := rand.Read(buf); err != nil {
		return tokenshare.Token{}, err
	}

	return tokenshare.Token{ID: buf, T: time.Now()}, nil
}

func (d *database) insert(t tokenshare.Token) error {
	buf, err := tokenshare.Marshal(t)
	if err != nil {
		return err
	}

	return d.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(d.bucket))
		return bucket.Put(t.ID, buf)
	})
}

func (d *database) generate() (tokenshare.Token, error) {
	t, err := d.new()
	if err != nil {
		return tokenshare.Token{}, err
	}

	if err := d.insert(t); err != nil {
		return tokenshare.Token{}, err
	}

	return t, nil
}

func (d *database) poke(id []byte) (tokenshare.Token, bool, error) {
	exists := false
	var tok tokenshare.Token

	err := d.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(d.bucket))

		v := bucket.Get(id)
		if v == nil {
			return nil
		}

		var err error
		tok, err = tokenshare.Unmarshal(v)
		if err != nil {
			return err
		}

		exists = true
		return nil
	})

	return tok, exists, err
}

func (d *database) update(id []byte, token tokenshare.Token) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(d.bucket))

		d := bucket.Get(id)
		if d == nil {
			return fmt.Errorf("no such token: %s", hex.EncodeToString(id))
		}
		t, err := tokenshare.Unmarshal(d)
		if err != nil {
			return err
		}

		t.Name = token.Name

		d, err = tokenshare.Marshal(t)
		if err != nil {
			return err
		}

		return bucket.Put(id, d)
	})
}

func (d *database) list() ([]byte, error) {
	var res []byte
	var marshalErr error

	if err := d.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(d.bucket))

		n := bucket.Stats().KeyN
		cursor := bucket.Cursor()
		toks := make([]tokenshare.Token, n)
		i := 0

		defer func() {
			res, marshalErr = tokenshare.MarshalList(toks)
		}()

		_, v := cursor.First()
		if v == nil {
			return nil
		}

		tok, err := tokenshare.Unmarshal(v)
		if err != nil {
			return err
		}
		toks[i] = tok
		i++

		for {
			_, v := cursor.Next()
			if v == nil {
				return nil
			}

			tok, err := tokenshare.Unmarshal(v)
			if err != nil {
				return err
			}

			toks[i] = tok
			i++

		}
	}); err != nil {
		return nil, err
	}

	return res, marshalErr
}

func (d *database) single(id []byte) ([]byte, error) {
	var res []byte

	err := d.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(d.bucket))
		res = bucket.Get(id)
		if res == nil {
			return tokenshare.NoSuchToken{}
		}

		return nil
	})

	return res, err
}
