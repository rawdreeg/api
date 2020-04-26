package models

import (
	"io/ioutil"
	"os"
	"path"
	"strings"

	"cloud.google.com/go/datastore"

	"github.com/hiconvo/api/errors"
	"github.com/hiconvo/api/models/read"
)

func swapKeys(keyList []*datastore.Key, oldKey, newKey *datastore.Key) []*datastore.Key {
	for i := range keyList {
		if keyList[i].Equal(oldKey) {
			keyList[i] = newKey
		}
	}

	// Remove duplicates
	var clean []*datastore.Key
	seen := map[string]struct{}{}
	for i := range keyList {
		keyString := keyList[i].String()
		if _, hasVal := seen[keyString]; !hasVal {
			seen[keyString] = struct{}{}
			clean = append(clean, keyList[i])
		}
	}

	return clean
}

func swapReadUserKeys(readList []*read.Read, oldKey, newKey *datastore.Key) []*read.Read {
	var clean []*read.Read
	seen := map[string]struct{}{}
	for i := range readList {
		keyString := readList[i].UserKey.String()
		if _, isSeen := seen[keyString]; !isSeen {
			seen[keyString] = struct{}{}

			if readList[i].UserKey.Equal(oldKey) {
				readList[i].UserKey = newKey
			}

			clean = append(clean, readList[i])
		}
	}

	return clean
}

func readStringFromFile(file string) string {
	op := errors.Opf("models.readStringFromFile(file=%s)", file)

	wd, err := os.Getwd()
	if err != nil {
		// This function should only be run at startup time, so we
		// panic if it fails.
		panic(errors.E(op, err))
	}

	var basePath string
	if strings.HasSuffix(wd, "models") || strings.HasSuffix(wd, "integ") {
		// This package is the cwd, so we need to go up one dir to resolve the
		// layouts and includes dirs consistently.
		basePath = "../models/content"
	} else {
		basePath = "./models/content"
	}

	b, err := ioutil.ReadFile(path.Join(basePath, file))
	if err != nil {
		panic(err)
	}

	return string(b)
}

func MapReadsToUserPartials(r read.Readable, users []*User) []*UserPartial {
	reads := r.GetReads()
	var userPartials []*UserPartial
	for i := range reads {
		for j := range users {
			if users[j].Key.Equal(reads[i].UserKey) {
				userPartials = append(userPartials, MapUserToUserPartial(users[j]))
				break
			}
		}
	}

	return userPartials
}
