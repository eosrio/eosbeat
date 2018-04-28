<h1 align="center">
  <br>
  eosbeat
  <br>
</h1>
<h3 align="center">
Network metrics tool for EOS.IO networks based on Elastic Beats
</h3>

*Made with :hearts: by [EOS Rio](https://steemit.com/@eosrio)*

# Introduction

Eosbeat will allow nodes on a EOS.IO network to consume data from an api that provides optimized topology information, thus being able to automatically set the optimal peers of a node.

Eosbeat works by collecting response times from your server to all other nodes on your network, as defined by the nodes.json file. The data is sent to EOS Rio server, which will be processed later to provide an overall view of network performance from EOS.IO nodes.
The ideal scenario is to have eosbeat running on all participating nodes of a network.

We will be soon releasing a dashboard page with all data collected from the current test networks.

A central elasticsearch server is useful to streamline the development process, but as soon as the topology optimization algorithm is tuned we can move it to a smart contract based solution.

# Running

To run on your node simply download the latest release, there's no need to install any dependencies.
```
wget https://github.com/eosrio/eosbeat/releases/download/v0.2.0/eosbeat.tar.gz
tar -xzvf eosbeat.tar.gz
cd release
```
Edit `eosbeat.conf.yml` and set username and password

The current nodes will be loaded from the node.json file, which is currently set to the Jungle Testnet

```
mv eosbeat.conf.yml eosbeat.yml
./run-eosbeat.sh
```

Stop with `./stop-eosbeat.sh`

# Building Instructions

Ensure that this folder is at the following location:
`${GOPATH}/src/github.com/eosrio/eosbeat`

### Requirements

* [Golang](https://golang.org/dl/) 1.7

### Init Project
To get running with Eosbeat and also install the
dependencies, run the following command:

```
make setup
```

It will create a clean git history for each major step. Note that you can always rewrite the history if you wish before pushing your changes.

To push Eosbeat in the git repository, run the following commands:

```
git remote set-url origin https://github.com/eosrio/eosbeat
git push origin master
```

For further development, check out the [beat developer guide](https://www.elastic.co/guide/en/beats/libbeat/current/new-beat.html).

### Build

To build the binary for Eosbeat run the command below. This will generate a binary
in the same directory with the name eosbeat.

```
make
```

### Run

To run Eosbeat with debugging output enabled, run:

```
./eosbeat -c eosbeat.yml -e -d "*"
```


### Test

To test Eosbeat, run the following command:

```
make testsuite
```

alternatively:
```
make unit-tests
make system-tests
make integration-tests
make coverage-report
```

The test coverage is reported in the folder `./build/coverage/`

### Update

Each beat has a template for the mapping in elasticsearch and a documentation for the fields
which is automatically generated based on `fields.yml` by running the following command.

```
make update
```


### Cleanup

To clean  Eosbeat source code, run the following commands:

```
make fmt
make simplify
```

To clean up the build directory and generated artifacts, run:

```
make clean
```


### Clone

To clone Eosbeat from the git repository, run the following commands:

```
mkdir -p ${GOPATH}/src/github.com/eosrio/eosbeat
git clone https://github.com/eosrio/eosbeat ${GOPATH}/src/github.com/eosrio/eosbeat
```


For further development, check out the [beat developer guide](https://www.elastic.co/guide/en/beats/libbeat/current/new-beat.html).


## Packaging

The beat frameworks provides tools to crosscompile and package your beat for different platforms. This requires [docker](https://www.docker.com/) and vendoring as described above. To build packages of your beat, run the following command:

```
make package
```

This will fetch and create all images required for the build process. The whole process to finish can take several minutes.
