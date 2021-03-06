// Copyright 2014 beego Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

var (
	DEFAULT_SECTION = "default"   // default section means if some ini items not in a section, make them in default section,
	bNumComment     = []byte{'#'} // number signal
	bSemComment     = []byte{';'} // semicolon signal
	bEmpty          = []byte{}
	bEqual          = []byte{'='} // equal signal
	bDQuote         = []byte{'"'} // quote signal
	sectionStart    = []byte{'['} // section start signal
	sectionEnd      = []byte{']'} // section end signal
	lineBreak       = "\n"
)

// IniConfig implements Config to parse ini file.
type IniConfig struct {
}

// ParseFile creates a new Config and parses the file configuration from the named file.
func (ini *IniConfig) Parse(name string) (ConfigContainer, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	cfg := &IniConfigContainer{
		file.Name(),
		make(map[string]map[string]string),
		make(map[string]string),
		make(map[string]string),
		sync.RWMutex{},
	}
	cfg.Lock()
	defer cfg.Unlock()
	defer file.Close()

	var comment bytes.Buffer
	buf := bufio.NewReader(file)
	section := DEFAULT_SECTION
	for {
		line, _, err := buf.ReadLine()
		if err == io.EOF {
			break
		}
		if bytes.Equal(line, bEmpty) {
			continue
		}
		line = bytes.TrimSpace(line)

		var bComment []byte
		switch {
		case bytes.HasPrefix(line, bNumComment):
			bComment = bNumComment
		case bytes.HasPrefix(line, bSemComment):
			bComment = bSemComment
		}
		if bComment != nil {
			line = bytes.TrimLeft(line, string(bComment))
			line = bytes.TrimLeftFunc(line, unicode.IsSpace)
			comment.Write(line)
			comment.WriteByte('\n')
			continue
		}

		if bytes.HasPrefix(line, sectionStart) && bytes.HasSuffix(line, sectionEnd) {
			section = strings.ToLower(string(line[1 : len(line)-1])) // section name case insensitive
			if comment.Len() > 0 {
				cfg.sectionComment[section] = comment.String()
				comment.Reset()
			}
			if _, ok := cfg.data[section]; !ok {
				cfg.data[section] = make(map[string]string)
			}
			continue
		}

		if _, ok := cfg.data[section]; !ok {
			cfg.data[section] = make(map[string]string)
		}
		keyValue := bytes.SplitN(line, bEqual, 2)
		val := bytes.TrimSpace(keyValue[1])
		if bytes.HasPrefix(val, bDQuote) {
			val = bytes.Trim(val, `"`)
		}

		key := string(bytes.TrimSpace(keyValue[0])) // key name case insensitive
		key = strings.ToLower(key)
		cfg.data[section][key] = string(val)
		if comment.Len() > 0 {
			cfg.keyComment[section+"."+key] = comment.String()
			comment.Reset()
		}

	}
	return cfg, nil
}

func (ini *IniConfig) ParseData(data []byte) (ConfigContainer, error) {
	// Save memory data to temporary file
	tmpName := path.Join(os.TempDir(), "beego", fmt.Sprintf("%d", time.Now().Nanosecond()))
	os.MkdirAll(path.Dir(tmpName), os.ModePerm)
	if err := ioutil.WriteFile(tmpName, data, 0655); err != nil {
		return nil, err
	}
	return ini.Parse(tmpName)
}

// A Config represents the ini configuration.
// When set and get value, support key as section:name type.
type IniConfigContainer struct {
	filename       string
	data           map[string]map[string]string // section=> key:val
	sectionComment map[string]string            // section : comment
	keyComment     map[string]string            // id: []{comment, key...}; id 1 is for main comment.
	sync.RWMutex
}

// Bool returns the boolean value for a given key.
func (c *IniConfigContainer) Bool(key string) (bool, error) {
	return strconv.ParseBool(c.getdata(strings.ToLower(key)))
}

// DefaultBool returns the boolean value for a given key.
// if err != nil return defaltval
func (c *IniConfigContainer) DefaultBool(key string, defaultval bool) bool {
	if v, err := c.Bool(key); err != nil {
		return defaultval
	} else {
		return v
	}
}

// Int returns the integer value for a given key.
func (c *IniConfigContainer) Int(key string) (int, error) {
	return strconv.Atoi(c.getdata(strings.ToLower(key)))
}

// DefaultInt returns the integer value for a given key.
// if err != nil return defaltval
func (c *IniConfigContainer) DefaultInt(key string, defaultval int) int {
	if v, err := c.Int(key); err != nil {
		return defaultval
	} else {
		return v
	}
}

// Int64 returns the int64 value for a given key.
func (c *IniConfigContainer) Int64(key string) (int64, error) {
	return strconv.ParseInt(c.getdata(strings.ToLower(key)), 10, 64)
}

// DefaultInt64 returns the int64 value for a given key.
// if err != nil return defaltval
func (c *IniConfigContainer) DefaultInt64(key string, defaultval int64) int64 {
	if v, err := c.Int64(key); err != nil {
		return defaultval
	} else {
		return v
	}
}

// Float returns the float value for a given key.
func (c *IniConfigContainer) Float(key string) (float64, error) {
	return strconv.ParseFloat(c.getdata(strings.ToLower(key)), 64)
}

// DefaultFloat returns the float64 value for a given key.
// if err != nil return defaltval
func (c *IniConfigContainer) DefaultFloat(key string, defaultval float64) float64 {
	if v, err := c.Float(key); err != nil {
		return defaultval
	} else {
		return v
	}
}

// String returns the string value for a given key.
func (c *IniConfigContainer) String(key string) string {
	key = strings.ToLower(key)
	return c.getdata(strings.ToLower(key))
}

// DefaultString returns the string value for a given key.
// if err != nil return defaltval
func (c *IniConfigContainer) DefaultString(key string, defaultval string) string {
	if v := c.String(key); v == "" {
		return defaultval
	} else {
		return v
	}
}

// Strings returns the []string value for a given key.
func (c *IniConfigContainer) Strings(key string) []string {
	return strings.Split(c.String(key), ";")
}

// DefaultStrings returns the []string value for a given key.
// if err != nil return defaltval
func (c *IniConfigContainer) DefaultStrings(key string, defaultval []string) []string {
	if v := c.Strings(key); len(v) == 0 {
		return defaultval
	} else {
		return v
	}
}

// GetSection returns map for the given section
func (c *IniConfigContainer) GetSection(section string) (map[string]string, error) {
	if v, ok := c.data[section]; ok {
		return v, nil
	} else {
		return nil, errors.New("not exist setction")
	}
}

// SaveConfigFile save the config into file
func (c *IniConfigContainer) SaveConfigFile(filename string) (err error) {
	// Write configuration file by filename.
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := bytes.NewBuffer(nil)
	for section, dt := range c.data {
		// Write section comments.
		if v, ok := c.sectionComment[section]; ok {
			if _, err = buf.WriteString(string(bNumComment) + v + lineBreak); err != nil {
				return err
			}
		}

		if section != DEFAULT_SECTION {
			// Write section name.
			if _, err = buf.WriteString(string(sectionStart) + section + string(sectionEnd) + lineBreak); err != nil {
				return err
			}
		}

		for key, val := range dt {
			if key != " " {
				// Write key comments.
				if v, ok := c.keyComment[key]; ok {
					if _, err = buf.WriteString(string(bNumComment) + v + lineBreak); err != nil {
						return err
					}
				}

				// Write key and value.
				if _, err = buf.WriteString(key + string(bEqual) + val + lineBreak); err != nil {
					return err
				}
			}
		}

		// Put a line between sections.
		if _, err = buf.WriteString(lineBreak); err != nil {
			return err
		}
	}

	if _, err = buf.WriteTo(f); err != nil {
		return err
	}
	return nil
}

// WriteValue writes a new value for key.
// if write to one section, the key need be "section::key".
// if the section is not existed, it panics.
func (c *IniConfigContainer) Set(key, value string) error {
	c.Lock()
	defer c.Unlock()
	if len(key) == 0 {
		return errors.New("key is empty")
	}

	var (
		section, k string
		sectionKey []string = strings.Split(key, "::")
	)

	if len(sectionKey) >= 2 {
		section = sectionKey[0]
		k = sectionKey[1]
	} else {
		section = DEFAULT_SECTION
		k = sectionKey[0]
	}

	if _, ok := c.data[section]; !ok {
		c.data[section] = make(map[string]string)
	}
	c.data[section][k] = value
	return nil
}

// DIY returns the raw value by a given key.
func (c *IniConfigContainer) DIY(key string) (v interface{}, err error) {
	if v, ok := c.data[strings.ToLower(key)]; ok {
		return v, nil
	}
	return v, errors.New("key not find")
}

// section.key or key
func (c *IniConfigContainer) getdata(key string) string {
	c.RLock()
	defer c.RUnlock()
	if len(key) == 0 {
		return ""
	}

	var (
		section, k string
		sectionKey []string = strings.Split(key, "::")
	)
	if len(sectionKey) >= 2 {
		section = sectionKey[0]
		k = sectionKey[1]
	} else {
		section = DEFAULT_SECTION
		k = sectionKey[0]
	}
	if v, ok := c.data[section]; ok {
		if vv, ok := v[k]; ok {
			return vv
		}
	}
	return ""
}

func init() {
	Register("ini", &IniConfig{})
}
