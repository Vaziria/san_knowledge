# Web Machine Shell Protocol.
1. give explanation that i have machine and user and same session chat.
2. machine can execute with send chat in `execute` key. And User in `chat` key.
2. force chat response to json.

## Flow Example.
```mermaid
sequenceDiagram

participant shell as Shell
participant usr as User
participant pro as Protocol
participant web as Web Chat


usr->>pro: send task message
pro->>web: send WebchatRequest in json form
web-->>pro: give result WebchatResponse in json form
pro-->>usr: send chat result under key `chat` if not empty.
pro->>shell: execute in shell under key `execute` if not empty.
shell-->>pro: send result shell
pro-->>web: send WebchatResponse under `execute_result` in json form. and set from type machine.

```

## Webchat Response Rule.
```proto

message WebchatResponse {
    string      execute
    string      chat
}

enum FromType {
    FROM_TYPE_UNSPECIFIED
    FROM_TYPE_EXECUTE
    FROM_TYPE_USER
}

message WebchatRequest {
    string      execute_result
    string      chat
    FromType    from
}

```


