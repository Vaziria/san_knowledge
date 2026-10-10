# Autotrade

## General.
1. all autotrade configuration live in `./autotrade.yaml`

## Command Line.
1. `autotrade mcp run`, for running mcp.
2. `autotrade knowledge view`, for Preview the Knowledge.

## Claude Trigger.
1. `/autotrade`, run analytical and doing action.
    ```mermaid
    stateDiagram-v2
    direction LR
    
    state "Doing Analyze" as analyze
    state "Doing Action" as action
    state "Updating Autotrade Knowledge" as knowledge

    [*]-->analyze
    analyze-->action
    action-->knowledge
    knowledge-->[*]

    ```
