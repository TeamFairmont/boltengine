# Changelog

### 1.1.1
* Bugfix in allow/deny list checking to ignore the following prefixes in URL paths: /request/, /task/, /work/, or /form/.
* Bugfix in extraConfigFolder.  Read path with or without trailing slash.

### 1.1.0
* Hook into SIGTERM and perform engine shutdown
* Added engine.advanced.queuePrefix config option to allow multiple engines sharing a MQ and/or redis server without naming conflicts
* Engine now listens on a BOLT_WORKER_ERROR queue and logs worker errors that get sent back to MQ

### 1.0.1
* Initial public release
