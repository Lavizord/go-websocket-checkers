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
            OperatorName VARCHAR(255),         -- Name of the operator
            OperatorGameName VARCHAR(255),    -- Name of the game for the operator
            GameName VARCHAR(255),             -- Internal game name.
            Active BOOLEAN DEFAULT TRUE,        -- Whether the operator is active or not, default to TRUE (1)
            GameBaseUrl VARCHAR(255),             -- gamelaunch base url
            OperatorWalletBaseUrl VARCHAR(255),
            WinFactor DECIMAL(5,4)                      
        );

        -- Insert a row into the table after creating it
        INSERT INTO operators (OperatorName, OperatorGameName, GameName, GameBaseUrl, OperatorWalletBaseUrl, WinFactor)
        VALUES (
            'SokkerDuel',
            'damasSokkerDuel',
            'BatalhaDasDamas',
            'https://s3.eu-central-1.amazonaws.com/play.retromindgames.pt/games/damasSokkerDuel/index.html',
            'https://tt2.sokkerduel.com',
            0.9
        ),
        (
            'TestOp',
            'damasSokkerDuel',
            'BatalhaDasDamas',
            'https://s3.eu-central-1.amazonaws.com/play.retromindgames.pt/games/damasSokkerDuel/index.html',
            '',
            0.9
        );
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS sessions (
    SessionId UUID PRIMARY KEY,  -- Unique session ID
    Token VARCHAR(255),          -- Session token
    PlayerName VARCHAR(255),    -- Player's name
    Currency VARCHAR(10),        -- Currency code (e.g., EUR, USD)
    OperatorBaseUrl VARCHAR(255),  -- Operator's base URL
    CreatedAt TIMESTAMP DEFAULT CURRENT_TIMESTAMP,  -- Timestamp when the session was created
    OperatorName VARCHAR(255),      -- Operator name (from OperatorIdentifier)
    OperatorGameName VARCHAR(255),  -- Operator game name (from OperatorIdentifier)
    GameName VARCHAR(255)                    -- Internal game name (from OperatorIdentifier)
);

/*
 Games table, to store the games when they are finished.
 Operator name identifies the casino that is operating the game.
*/
CREATE TABLE IF NOT EXISTS games (
    ID UUID PRIMARY KEY,
    OperatorName VARCHAR(100),
    OperatorGameName VARCHAR(100) ,
    GameName    VARCHAR(100) ,
    StartDate TIMESTAMP,
    EndDate TIMESTAMP,
    NumMoves INT,
    Moves JSONB DEFAULT '[]',       
    BetAmount DECIMAL(10,2) CHECK (BetAmount >= 0),  
    Winner UUID ,                     
    WinFactor DECIMAL(5,4), 
    GameOverReason VARCHAR(50),
    GamePlayers JSONB DEFAULT '[]'  
);

CREATE TABLE IF NOT EXISTS transactions (
    TransactionID UUID PRIMARY KEY,    
    SessionID UUID ,                   
    Type VARCHAR(50)  CHECK (Type IN ('bet', 'win')), 
    Amount INTEGER  CHECK (Amount >= 0),  
    Currency VARCHAR(10) ,  
    Platform VARCHAR(100) , -- Platform name
    Operator VARCHAR(100) , -- Operator name (e.g., SokkerDuel)
    Client VARCHAR(255) ,       -- Client ID ( for Sokker its player name).
    Game VARCHAR(100) ,         -- Internal game name
    Status VARCHAR(100) ,               
    Description VARCHAR(600),           -- Description (e.g., "Insufficient Funds" or "OK")
    RoundID UUID,                       -- Foreign key to the round / game
    Timestamp TIMESTAMP  DEFAULT CURRENT_TIMESTAMP  -- Timestamp in UTC
);