# OpenAI API examples

Workflows that make the tower an OpenAI-compatible chat API answered by the
browser chats (ChatGPT, Gemini, Claude on claude.ai) through San Remote (the
user, 2026-11-03: "Can I have open ai compatible node, that can configure
port and listen host, and can connected to any ai chat browser node in tower
?"). They were built in a tower and exported (ExportWorkflows), so they are
exactly the tower's format. See the tower's README (Workflows › OpenAI API
and OpenAI Reply) for the details.

| File | Workflow | Address | How a conversation reaches the chat |
| --- | --- | --- | --- |
| `openai-api-chatgpt.workflow.json` | OpenAI API - ChatGPT | http://127.0.0.1:7480/v1 | Method 1: each request's whole conversation as one prompt, in a new Temporary ChatGPT chat. Stateless, like the real API. |
| `openai-api-chatgpt-session.workflow.json` | OpenAI API - ChatGPT Session | http://127.0.0.1:7481/v1 | Method 2: a Browser Chat Session keeps one ChatGPT chat (in your history). A conversation's first request starts it with all of it; each next request sends only the newest message into it. A request with no assistant message (a new conversation) starts a new kept chat. |
| `openai-api-router.workflow.json` | OpenAI API - Router | http://127.0.0.1:7482/v1 | Method 1, on the site the request's `model` names: `chatgpt`, `gemini` or `claude` (a model starting with gemini or claude goes there); any other model, or none, goes to ChatGPT. |

Each is OpenAI API → (Regex Filters on Model, in the router) → ChatGPT Ask /
Gemini Ask / Claude Ask (Browser), with Messages and Tools linked → OpenAI
Reply, with Answer and Tool Calls linked. They use different ports, so all
three can be on at once. Tool calls and images in the request work on each
(see below).

## Import and switch on

1. The tower needs a San Remote connection (Connections page) to `san_remote
   api` (http://127.0.0.1:7478, or the tower's own Browser Remote Server),
   with Chrome attached and signed in to the sites. Name it `San Remote` and
   the import picks it by name; otherwise it takes the only San Remote
   connection, or the import notes say to pick one in the ask steps.
2. Workflows page › Import, and pick the file(s). Or through the API:

   ```sh
   python -c "import json,sys; print(json.dumps({'json': open(sys.argv[1], encoding='utf-8').read()}))" openai-api-chatgpt.workflow.json > body.json
   curl http://127.0.0.1:7477/san.tower.v1.WorkflowService/ImportWorkflows -H 'Content-Type: application/json' -d @body.json
   ```

3. Switch the workflow on. Its OpenAI API step then says `Listening on
   http://127.0.0.1:7480/v1` (or why not: a port in use, another workflow
   on it).

Set an API Key on the OpenAI API step to make clients send `Authorization:
Bearer <key>`; change Host only if other machines must reach it (anyone who
can then uses your browser accounts, and Windows Firewall asks once for
san_tower.exe).

## Use it

```sh
curl http://127.0.0.1:7480/v1/models
curl http://127.0.0.1:7480/v1/chat/completions -H 'Content-Type: application/json' \
  -d '{"model": "browser-chat", "messages": [{"role": "user", "content": "Reply with the single word: pong"}]}'
# streamed (Server-Sent Events)
curl -N http://127.0.0.1:7480/v1/chat/completions -H 'Content-Type: application/json' \
  -d '{"stream": true, "messages": [{"role": "user", "content": "Reply with the single word: pong"}]}'
# the router: the model picks the site
curl http://127.0.0.1:7482/v1/chat/completions -H 'Content-Type: application/json' \
  -d '{"model": "gemini", "messages": [{"role": "user", "content": "Reply with the single word: pong"}]}'
```

Any OpenAI client works with the base URL and any API key (or the step's):

```python
from openai import OpenAI

client = OpenAI(base_url="http://127.0.0.1:7482/v1", api_key="not-needed")
r = client.chat.completions.create(model="claude", messages=[{"role": "user", "content": "hi"}])
print(r.choices[0].message.content)
for chunk in client.chat.completions.create(model="chatgpt", stream=True, messages=[{"role": "user", "content": "hi"}]):
    if chunk.choices and chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
```

Requests to one step are answered one at a time (a browser chat answers one
at a time), each within the step's Timeout (10m).

## Tool calls

The web chats have no function calling a program can use, so the Ask tells
the chat the request's `tools` (a prompt protocol: a call comes back as a
JSON object in a code block), reads the calls in its answer, and OpenAI
Reply answers them as OpenAI's `tool_calls` (`finish_reason` tool_calls,
streamed too). The client runs the tools and sends their results as `tool`
messages; the chat gets them as text. `tool_choice` (auto, none, required,
a named function) and `parallel_tool_calls` are kept; a broken call is
corrected once. In the session example the kept chat is told the tools
once, then gets a one-line reminder with each tool result.

```python
import json
from datetime import datetime, timezone
from openai import OpenAI

client = OpenAI(base_url="http://127.0.0.1:7482/v1", api_key="not-needed")
tools = [{"type": "function", "function": {"name": "get_time", "description": "The current time in a time zone.",
          "parameters": {"type": "object", "properties": {"tz": {"type": "string"}}, "required": ["tz"]}}}]
messages = [{"role": "user", "content": "What time is it now in UTC? Use the tools."}]
while True:
    m = client.chat.completions.create(model="chatgpt", messages=messages, tools=tools).choices[0].message
    if not m.tool_calls:
        print(m.content)
        break
    messages.append(m.model_dump(exclude_none=True))
    for c in m.tool_calls:  # here only get_time
        now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M UTC")
        messages.append({"role": "tool", "tool_call_id": c.id, "content": now})
```

A chat sometimes answers by itself instead of calling (it is a prompt, not
a contract): a session (method 2) and tool descriptions that say when to
use each help. Each step of the loop is a run, with a new Temporary chat in method 1, so
a long agent loop is slow (about 20 s a turn).

## Images

An `image_url` part, a `data:` URL (base64) or an http(s) link, goes to the
chat: San Remote attaches it in the composer before the prompt (png, jpeg,
webp or gif, at most 20 MB, 10 per ask). Into a new chat all of the
conversation's images go; into a session's kept chat only the newest
turn's. Gemini asks each account once to agree to "Creating content from
images and files" before its first upload: press Agree in Gemini yourself
first, or its asks with images fail saying so.

```python
import base64
img = base64.b64encode(open("tes.png", "rb").read()).decode()
r = client.chat.completions.create(model="claude", messages=[{"role": "user", "content": [
    {"type": "text", "text": "What number is in this image? Answer with only it."},
    {"type": "image_url", "image_url": {"url": "data:image/png;base64," + img}}]}])
print(r.choices[0].message.content)  # 4721
```

Images the chat makes do not come back: the answer is text.
