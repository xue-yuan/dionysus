from datetime import datetime, timedelta, timezone

import jwt

import config


def generate(user_id, hours=config.TOKEN_TTL):
    token = jwt.encode(
        {
            "user_id": user_id,
            "exp": datetime.now(tz=timezone.utc) + timedelta(hours=hours),
        },
        config.SECRET_KEY,
    )

    return token


def validate(token):
    try:
        return jwt.decode(token, config.SECRET_KEY, algorithms=["HS256"])
    except jwt.ExpiredSignatureError:
        return ""
