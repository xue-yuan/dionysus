from enum import IntEnum, auto


class ServerError(IntEnum):
    UNDOCUMENTED_EXCEPTION = 10000
    DATABASE_INTEGRITY_ERROR = auto()


class ClientError(IntEnum):
    INVALID_TOKEN = 20000
    NOT_AUTHENTICATED = auto()
    INVALID_CREDENTIALS = auto()
    DUPLICATE_USERNAME = auto()
    INCORRECT_USERNAME_OR_PASSWORD = auto()
    INVALID_USER_OPERATION = auto()
