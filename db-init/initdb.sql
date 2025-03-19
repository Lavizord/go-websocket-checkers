DO $$
BEGIN
    -- Check if the table exists
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_name = 'operators'
    ) THEN
        -- Create the table if it doesn't exist
        CREATE TABLE operators (
            ID SERIAL PRIMARY KEY,
            OperatorName VARCHAR(255) NOT NULL,         -- Name of the operator
            OperatorGameName VARCHAR(255) NOT NULL,    -- Name of the game for the operator
            GameName VARCHAR(255) NOT NULL,             -- Internal game name.
            Active BOOLEAN NOT NULL DEFAULT TRUE,        -- Whether the operator is active or not, default to TRUE (1)
            GameBaseUrl VARCHAR(255) NOT NULL,                       -- gamelaunch base url
            OperatorWalletBaseUrl VARCHAR(255) NOT NULL
        );

        -- Insert a row into the table after creating it
        INSERT INTO operators (OperatorName, OperatorGameName, GameName, GameBaseUrl, OperatorWalletBaseUrl)
        VALUES (
            'SokkerDuel',
            'damasSokkerDuel',
            'BatalhaDasDamas',
            'https://miguelclg.github.io/Damas/docs',
            'http://88.99.49.131:3000'
        );
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS sessions (
    SessionId VARCHAR(255) PRIMARY KEY,  -- Unique session ID
    Token VARCHAR(255) NOT NULL,          -- Session token
    PlayerName VARCHAR(255) NOT NULL,    -- Player's name
    Balance BIGINT NOT NULL,              -- Player's balance (in cents or smallest currency unit)
    Currency VARCHAR(10) NOT NULL,        -- Currency code (e.g., EUR, USD)
    OperatorBaseUrl VARCHAR(255) NOT NULL,  -- Operator's base URL
    CreatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,  -- Timestamp when the session was created
    OperatorName VARCHAR(255) NOT NULL,  -- Operator name (from OperatorIdentifier)
    OperatorGameName VARCHAR(255) NOT NULL,  -- Operator game name (from OperatorIdentifier)
    GameName VARCHAR(255) NOT NULL       -- Internal game name (from OperatorIdentifier)
);

/*
 Games table, to store the games when they are finished.
 Operator name identifies the casino that is operating the game.
*/
CREATE TABLE IF NOT EXISTS games (
    ID SERIAL PRIMARY KEY,
    OperatorName VARCHAR(100) NOT NULL,
    OperatorGameName VARCHAR(100) NOT NULL,
    GameName    VARCHAR(100) NOT NULL,
    StartDate TIMESTAMP NOT NULL,
    EndDate TIMESTAMP,
    Moves JSONB NOT NULL DEFAULT '[]',       -- JSON array for moves
    BetAmount DECIMAL(10,2) NOT NULL CHECK (bet_amount >= 0),
    Winner VARCHAR(255),                     -- Store winner's name or ID
    GamePlayers JSONB NOT NULL DEFAULT '[]'  -- JSON array for players
);

CREATE TABLE IF NOT EXISTS transactions (
    TransactionID SERIAL PRIMARY KEY, -- Unique ID for each transaction
    SessionID VARCHAR(255) NOT NULL,  -- Session ID for the player
    Type VARCHAR(50) NOT NULL CHECK (Type IN ('bet', 'win'),  -- Type of transaction: bet or win
    Amount INTEGER NOT NULL CHECK (Amount >= 0),  -- Amount in cents
    Currency VARCHAR(10) NOT NULL,  -- Currency code (e.g., EUR, USD)
    Platform VARCHAR(100) NOT NULL, -- Platform name
    Operator VARCHAR(100) NOT NULL, -- Operator name (e.g., SokkerDuel)
    Client VARCHAR(255) NOT NULL,   -- Client ID (player ID)
    Game VARCHAR(100) NOT NULL,     -- Internal game name
    Status VARCHAR(100) NOT NULL,        -- HTTP status code
    Description VARCHAR(600),           -- Description (e.g., "Insufficient Funds" or "OK")
    RoundID VARCHAR(255),                -- Foreign key to the round / game
    Timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP  -- Timestamp in UTC
);