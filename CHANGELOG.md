# Changelog

### 1.1.0
* Hook into SIGTERM and perform engine shutdown
* Added engine.advanced.queuePrefix config option to allow multiple engines sharing a MQ and/or redis server without naming conflicts
* Engine now listens on a BOLT_WORKER_ERROR queue and logs worker errors that get sent back to MQ

### 1.0.1
* Initial public release