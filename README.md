# Bolt Engine  
The open source Bolt Engine is a scalable API builder that uses an AMQP-compatible server to distribute and coordinate tasks to workers in any programming language. It provides a common authentication mechanism and input / output format that allows many disparate systems to be seamlessly integrated and exposed to API consumer applications. By providing performance contracts, worker stubs, failure models, and built-in documentation, the Bolt Engine enables application developers to rapidly create both frontend and backend in parallel with the knowledge that their application will scale well and utilize the same access patterns no matter what systems the backend utilizes. Documentation can be found here: https://docs.google.com/document/d/1lLQj5bPhtF5qB0U5MI9Wh72BNCVWznxiYe-WrRNZ_pc/edit?usp=sharing

## Screenshots
![api docs screenshot](https://github.com/TeamFairmont/boltengine/wiki/images/bolt-docs-ss.png)

![debug form screenshot](https://github.com/TeamFairmont/boltengine/wiki/images/bolt-debug-form.png)

## Quickstart
These instructions assume you already have a GoLang install configured in a Linux environment.

Install git, and download the bolt releases, and install RabbitMQ
```
# Install Git
sudo yum install git

# Download the bolt releases
mkdir ~/go/src/github.com/TeamFairmont -p
cd ~/go/src/github.com/TeamFairmont

git clone https://github.com/TeamFairmont/boltshared
git clone https://github.com/TeamFairmont/boltengine
cd boltshared
go get ./...
cd ../boltengine
go get ./…

# Install RabbitMQ, start it, and enable it on boot
sudo yum install rabbitmq-server cyrus-sasl-devel
sudo systemctl start rabbitmq-server
sudo systemctl enable rabbitmq-server.service
```

Run the API
```
cd ~/go/src/github.com/TeamFairmont/boltengine
go run api.go

# In a web browser, visit http://localhost:8888/docs
# Press ctrl-C to stop running the API.
# See the Documentation section below for detailed instructions on building the API to a binary executable and running as a service.
```

Create a custom configuration
```
sudo mkdir /etc/bolt
sudo cp ~/go/src/github.com/TeamFairmont/boltengine/etc/bolt/config.json /etc/bolt/
# Edit the contents of /etc/bolt/config.json
# The API will read use the custom configuration the next time it is run. 
# See the Documentation section below for detailed instructions on configuring the engine.  
```

Create workers

See boltsdk's readme for instructions and sample code using Bolt's Go SDK.
https://github.com/TeamFairmont/boltsdk-go
```
cd ~/go/src/github.com/TeamFairmont
go get github.com/TeamFairmont/boltsdk-go/boltsdk
```

## Documentation
Additional documentation can be found here: https://docs.google.com/document/d/1lLQj5bPhtF5qB0U5MI9Wh72BNCVWznxiYe-WrRNZ_pc/edit?usp=sharing
