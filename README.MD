# SQS Message Mover
SQS Mover lets you move messages from one AWS SQS queue to another. Useful when you need to move deadletter queue messages back into the original queue.

## Installation

### MAC:
```
brew tap ssuareza/brew git@github.com:ssuareza/homebrew-brew
brew install ssuareza/brew/sqs -f
```

### Linux:
```
curl -sLo sqs curl -sLo sqs https://github.com/ssuareza/sqsmover/releases/download/v0.0.1/sqs-v0.0.1-linux-amd64
chmod +x sqs
sudo mv sqs /usr/local/bin/
```

## Upgrade

### MAC:
```
brew update
brew upgrade ssuareza/brew/sqs -f
```

### Linux:
Follow installation steps.

## Usage
```
sqs --help

usage: sqs --source=SOURCE --destination=DESTINATION [<flags>]

Flags:
    --help                         Show context-sensitive help (also try --help-long and --help-man).
    -s, --source=SOURCE            Source queue to move messages from
    -d, --destination=DESTINATION  Destination queue to move messages to
    -p, --profile="default"        AWS Profile for source and destination queues
    -r, --region="us-east-1"       AWS Region for source and destination queues
```

### Examples:

Region will default to `us-east-1`, you can also override it with `--region` flag.

```
sqs --source=my_source_queue_name --destination=my_destination_queuename
sqs --source=my_source_queue_name --destination=my_destination_queuename --region=us-east-1

-- shorthand
sqs -s my_source_queue_name -d my_destination_queuename -r us-east-1
```



