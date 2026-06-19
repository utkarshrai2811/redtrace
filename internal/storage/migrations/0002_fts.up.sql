-- Full-text / substring search over captured exchanges. The trigram tokenizer
-- gives Burp-style "contains" matching across URLs and raw request/response
-- bytes. Rows are maintained from Go when an exchange is stored.
CREATE VIRTUAL TABLE IF NOT EXISTS search_index USING fts5(
    request_id UNINDEXED,
    url,
    request_raw,
    response_raw,
    tokenize = 'trigram'
);
