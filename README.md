# boltengine
The Bolt engine is the heart of Fairmont, and provides a scalable API framework to build upon. If you are using the Bolt engine in a production environment, we highly recommend using a pre-compiled release. The instructions below only apply to those who wish to work in the engine source code. 

## Setup
These instructions assume you already have a GoLang install configured in a Linux environment.
Copy the contents of this repo's etc/bolt folder to /etc/bolt/ to work on custom configs.
```
sudo mkdir /etc/bolt
sudo cp ./etc/bolt/* /etc/bolt/

# The user bolt's api runs as will need read/write access to the config files.
# Values in /etc/bolt/config.json will be used to create a custom configuration.  You may customize it as necessary for your environment.
# You may also specify a path for additional config files in config.json's engine > extraConfigFolder.
# When the bolt api is started, it will look in the path specified by extraConfigFolder for any of the following files:
#  apiCalls.json, cache.json, commandMeta.json, engine.json, logging.json, security.json, and/or workerConfig.json
# If any of those files exist, their contents will be used to overwrite the existing config values, including any set by config.json
#
# You may wish to give different groups access to each json file, but the user running bolt will need to be able to access each file.
# This can be done with user or group permissions, or with ownership.
sudo chown username:groupname /etc/bolt/config.json
sudo chown username:groupname /path_to/additional_configs/filename.json
```

Install additional required packages:
```
# Install rabbitmq
sudo yum update
sudo yum install rabbitmq-server
sudo yum install cyrus-sasl-devel   # This fixes errors about sasl/sasl.h: No such file or directory
# Enable the service on boot
sudo systemctl enable rabbitmq-server.service
# Start the server
sudo systemctl start rabbitmq-server
```

Install redis for caching: http://redis.io/download

## Dev Testing
To grab all dependencies for unit tests
```
go get -t ./...
```
To execute unit tests (from the boltengine folder)
```
# To run tests, excluding vendored packages (preferred):
go test $(go list ./... | grep -v /vendor/)

# To run all tests:
go test ./...

# To run tests for a specific package with verbose logging:
go test ./packagename -v
```

## Running the engine
```
go run api.go    # The old version in master is: go run bolt.go
# Make a note of the host and port number for communicating with the engine.
```

Call handlers by visiting http(s)://host:port/form/handlername (ex: http://localhost:8888/form/v1/addProduct) in your browser.

## (Optional) Compile, set capabilities to bind low port numbers, and run the api as a binary
```
# Compile
go build api.go

# Set capability to bind to a privileged port (<1024)
sudo setcap cap_net_bind_service+ep ./api

# Make sure rabbitmq has been started
sudo systemctl start rabbitmq-server

# Start the server
./api
# If running as a systemd service, use
sudo systemctl restart bolt
```

## (Optional) Run Bolt as a systemd service that automatically restarts
```
# First, follow the previous steps for compiling and setting capabilities for binding privileged ports.  Starting the server isn't necessary, and will be done in the next steps.

# Copy the example bolt.service file to your systemd/system folder
sudo cp ./etc/systemd/system/bolt.service /etc/systemd/system/

# Update the file with the appropriate username, making sure the paths to your bolt api executable are correct
sudo vi /etc/systemd/system/bolt.service
        # Make changes as needed. Replace [username] with your username in User, WorkingDirectory, and ExecStart.  The square braces should be removed.

# Enable the script on boot
sudo systemctl enable bolt
        # Enter your password if/when prompted

# Run the message service and bolt engine
sudo systemctl start rabbitmq-server
sudo systemctl start bolt

# Check the status
systemctl status bolt -l

# Test that the bolt api restarts after a failure.
killall api
# The process may take approximately 30 seconds to restart.  
# Tailing the log file should show messages indicating the bolt engine
# has started, or wait 30 seconds and run the status command above.
tail -f /var/log/syslog     # On CentOS or RHEL:  tail -f /var/log/messages

# To quickly compile bolt and restart the service:
go build api.go; sudo setcap cap_net_bind_service+ep ./api; sudo systemctl restart rabbitmq-server; sudo systemctl restart bolt; sudo tail -f /var/log/messages
```

## (Optional) Setup Redis for caching API call return_value parameters as needed
```
# In a browser, follow instructions at http://redis.io/download for latest stable version 3.x
# As of this writing, the current version was 3.0.7
wget http://download.redis.io/releases/redis-3.0.7.tar.gz
tar xzf redis-3.0.7.tar.gz
cd redis-3.0.7
make

cd src
sudo cp redis-server redis-cli redis-sentinel redis-benchmark redis-check-aof redis-check-dump /usr/local/bin

cd ../utils
sudo ./install_server.sh

	Welcome to the redis service installer
	This script will help you easily set up a running redis server

	Please select the redis port for this instance: [6379] 
	Selecting default: 6379
	Please select the redis config file name [/etc/redis/6379.conf] 
	Selected default - /etc/redis/6379.conf
	Please select the redis log file name [/var/log/redis_6379.log] 
	Selected default - /var/log/redis_6379.log
	Please select the data directory for this instance [/var/lib/redis/6379] 
	Selected default - /var/lib/redis/6379
	Please select the redis executable path [] /usr/local/bin/redis-server
	Selected config:
	Port           : 6379
	Config file    : /etc/redis/6379.conf
	Log file       : /var/log/redis_6379.log
	Data dir       : /var/lib/redis/6379
	Executable     : /usr/local/bin/redis-server
	Cli Executable : /usr/local/bin/redis-cli
	Is this ok? Then press ENTER to go on or Ctrl-C to abort.
	Copied /tmp/6379.conf => /etc/init.d/redis_6379
	Installing service...
	Successfully added to chkconfig!
	Successfully added to runlevels 345!
	Starting Redis server...
	Installation successful!

sudo systemctl enable redis_6379
sudo systemctl start redis_6379

# System administrators should perform additional steps to secure Redis, such as setting a password, renaming flush and config commands, setting max connections and overcommit memory, disable transparent huge pages, etc.

# In /etc/bolt/config.json (or the cache.json file), include redis caching:
	"cache": {
		"type": "redis",
		"host": "localhost:6379",
		"pass": "your_pass_here",
		"timeoutMs": 5000
	}

# Also setup API calls to enable caching of return_value parameters as necessary
	"apiCalls": {
		"callName": {
			...
			"cache": {
				"enabled": true,
				"expirationTimeSec": 5
			}
			...
		}
	}
```
