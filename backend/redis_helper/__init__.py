import redis

import config


class Singleton(type):
    _instances = {}

    def __call__(cls, *args, **kwargs):
        if cls not in cls._instances:
            cls._instances[cls] = super(Singleton, cls).__call__(*args, **kwargs)
        return cls._instances[cls]


class Redis(metaclass=Singleton):

    def __init__(self):
        self.pool = redis.ConnectionPool(
            host=config.REDIS_HOST,
            port=config.REDIS_PORT,
            password=config.REDIS_PASSWORD,
        )

    @property
    def conn(self):
        if not hasattr(self, "_conn"):
            self._getConnection()
        return self._conn

    def _getConnection(self):
        self._conn = redis.Redis(connection_pool=self.pool)

    def sadd(self, name, *values):
        self.conn.sadd(name, *values)

    def smembers(self, name):
        return self.conn.smembers(name)

    def smismember(self, name, values):
        self.conn.smismember(name, values)

    # The unit of "ex" is seconds, can use either `int` or `timedelta`
    def set(self, name, value, ex=None):
        self.conn.set(
            name=name,
            value=value,
            ex=ex,
        )

    def get(self, name):
        return self.conn.get(
            name=name,
        )


def initialize():
    Redis()
