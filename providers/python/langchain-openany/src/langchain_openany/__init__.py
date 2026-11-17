"""LangChain chat model for OpenAny Chat Completions."""

from langchain_openany.chat_models import ChatOpenAny
from langchain_openany.credentials import OpenAnyCredentialsError
from langchain_openany.endpoint import DEFAULT_BASE_URL

__version__ = "0.1.0"
__all__ = ["ChatOpenAny", "DEFAULT_BASE_URL", "OpenAnyCredentialsError", "__version__"]
