# Knowledge Map
**AI Supposed Not Edit This File**
We must have knowledge graph map tool to research. Its purpose to mapping all knowledge so user and ai can collaborative.

## Genenal.
1. we use golang.
2. all data knowledge saved in `./knowledge_data`
3. all golang project related place in `./san_knowledge`
4. we use embeded database from `github.com/mstrYoda/goraphdb`
5. we have unified knowledge tool placed in `./san_knowledge/cmd/knowledge`.
    Its use `urfave/cli` latest.
6. dist version place in `./bin/knowledge.exe`

## `knowledge.exe` Tools
### General Command.
1. `knowledge.exe view`, open our custom [ui](#knowledge-ui)
2. `knowledge.exe explain query`, to search knowledge context.
3. `knowledge.exe mcp`, make knowledge connected to our ai chat session.
4. `knowledge.exe query rawcypher-query`, for running raw cypher. 

## Knowledge UI
1. show the graph
2. there is form query that same do `explain query`

# Knowledge
This is How We Transfer the knowledge to Graph.

## Domain Node.
This is used for what big domain for the knowledge. for example `Internet Marketing`, `Coding`, `Bussiness` and other.

for example:
    node with label `Coding` and with properties `node_type` `domain`


## How We Convert Things to Knowledge Graph.

### Document Knowledge.
1. we track every doc in `./docs`
2. Node
    - node type properties `doc`
        
        label:
        - title of the documents.
    
        that have properties:
        - `loc`, project relative file path
        - `summary`, short summary
        - `keyword`, important keyword
    
    - node type properties `doc_section`

        label:
        - section title of the documents.

        that have properties:
        - `loc`, project relative file path
        - `line_loc`, location line in the doc.
        - `summary`, short summary
        - `keyword`, important keyword

3. Edge.
    - edge `domain_of`
    - edge `section_of`


