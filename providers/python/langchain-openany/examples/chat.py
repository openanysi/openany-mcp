"""Text Chat Completions through ChatOpenAny.

This example is not executed by the tests and does not contain a key.

    export OPENANY_API_KEY=...          # raw key, not a Bearer header
    export OPENANY_MODEL=MODEL_ID_FROM_LISTING
    python examples/chat.py

No live call is part of package verification.
"""

import os
import sys

from langchain_openany import ChatOpenAny

model_id = os.environ.get("OPENANY_MODEL")
if not model_id:
    print("Set OPENANY_MODEL to a model id from the current OpenAny listing.", file=sys.stderr)
    raise SystemExit(1)

model = ChatOpenAny(model=model_id, max_tokens=32)
message = model.invoke("Reply with exactly: OpenAny connected")
print(message.content)
print(message.usage_metadata)
print(message.response_metadata.get("finish_reason"))
