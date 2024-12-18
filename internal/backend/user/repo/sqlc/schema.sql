-- name:  create :one
CREATETABLEusers(
    idVARCHAR(36)PRIMARYKEY,
    roleINTNOTNULL,
    loginTEXTNOTNULLUNIQUE,
    hashTEXTNOTNULL
);
