CREATE TABLE Transactions(
    date Date,
    timestamp DateTime,
    transaction String,
    fromA String,
    toA String,
    value UInt64,
    data String,
    typeTx String,
    blockNumber UInt64,
    signature String,
    publickey String,
    fee UInt64,
    realFee UInt64,
    nonce UInt64,
    intStatus UInt64,
    status String,
    isDelegate UInt8,
    delegateInfo String,
    delegate UInt64,
    delegateHash String,
    dataString String,
    abstractMethod String
) ENGINE = ReplacingMergeTree() ORDER BY (timestamp, transaction)