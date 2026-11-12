package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

/**
 * cacheVersion changes whenever the index's content or format does, so
 * indexes written by an older doxygo aren't reused.
 */
const cacheVersion = "doxygo-index-3"

/**
 * indexCache keeps symbol indexes on disk between runs. Doxygen runs the
 * filter once per file, and type-checking the whole module each time would
 * repeat the same work for every file.
 */
type indexCache struct {
	/** dir holds the cached indexes; "" disables caching. */
	dir string
}

/**
 * newIndexCache returns a cache in the user's cache directory, or a
 * disabled one if there is none.
 *
 * @return The cache.
 */
func newIndexCache() *indexCache {
	dir, err := os.UserCacheDir()
	if err != nil {
		return &indexCache{}
	}
	return &indexCache{dir: filepath.Join(dir, "doxygo")}
}

/**
 * load returns the index for the module m (and the package in extraDir),
 * from the cache if its source is unchanged since the index was cached,
 * else by type-checking it.
 *
 * @param m The module.
 * @param extraDir A package directory to index even if the module wouldn't
 * include it, or "".
 * @return The index, or an error if the source can't be read or parsed.
 */
func (c *indexCache) load(m *module, extraDir string) (*symbolIndex, error) {
	key, err := sourceKey(m, extraDir)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(c.dir, key+".json")
	if c.dir != "" {
		if data, err := os.ReadFile(path); err == nil {
			var index symbolIndex
			if json.Unmarshal(data, &index) == nil {
				return &index, nil
			}
		}
	}
	tm, err := typeCheck(m, extraDir)
	if err != nil {
		return nil, err
	}
	index := buildIndex(tm)
	if c.dir != "" {
		c.store(path, index)
	}
	return index, nil
}

/**
 * store writes index to path, atomically so that concurrent filters never
 * read a partial file. Failing to cache isn't an error.
 *
 * @param path The file to write.
 * @param index The index to cache.
 */
func (c *indexCache) store(path string, index *symbolIndex) {
	data, err := json.Marshal(index)
	if err != nil || os.MkdirAll(c.dir, 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(c.dir, "index-*.tmp")
	if err != nil {
		return
	}
	_, err = tmp.Write(data)
	if closeErr := tmp.Close(); err == nil && closeErr == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		os.Remove(tmp.Name())
	}
}

/**
 * sourceKey hashes everything the index depends on: the module's location
 * and every source file's name and content.
 *
 * @param m The module.
 * @param extraDir A package directory indexed as well, or "".
 * @return The hash, in hex, or an error if a file can't be read.
 */
func sourceKey(m *module, extraDir string) (string, error) {
	dirs, err := m.packageDirs(extraDir)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	io.WriteString(h, cacheVersion+"\x00"+m.root+"\x00"+m.path+"\x00")
	for _, dir := range dirs {
		files, err := sourceFiles(dir)
		if err != nil {
			return "", err
		}
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				return "", err
			}
			io.WriteString(h, file+"\x00")
			h.Write(data)
			h.Write([]byte{0})
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

/**
 * cacheFromEnv returns the cache the DOXYGO_CACHE environment variable
 * asks for: a directory, "off" for none, or unset for the user's cache
 * directory.
 *
 * @return The cache.
 */
func cacheFromEnv() *indexCache {
	switch dir := os.Getenv("DOXYGO_CACHE"); dir {
	case "":
		return newIndexCache()
	case "off":
		return &indexCache{}
	default:
		return &indexCache{dir: dir}
	}
}
