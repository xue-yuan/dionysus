from sqlmodel import text


def update_query():
    return text(
        f"""
            UPDATE ingredients SET name = :name WHERE id = :id
        """
    )


def delete_query():
    return text(
        f"""
            DELETE FROM ingredients WHERE id = :id
        """
    )
