# Event Cache

This installs is a simple file server from the test_images/event-cache package.
The local `events` dir is symlinked into the `kodata` dir.

Example local test (from the root of the project):

```shell
KO_DATA_PATH=./test/test_images/event-cache/kodata \
  go run ./test/test_images/event-cache
```

Then access the files path-ed exactly as they are rooted from
`resources/eventcache` dir:

```shell
curl localhost:8080/events/three.ce
```

Results in the contents of `./events/three.ce`.
