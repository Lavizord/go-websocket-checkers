DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_name = 'operators'
    ) THEN
        CREATE TABLE operators (
            ID SERIAL PRIMARY KEY,
            OperatorName VARCHAR(255),
            OperatorGameName VARCHAR(255),
            GameName VARCHAR(255),        
            Active BOOLEAN DEFAULT TRUE, 
            GameBaseUrl VARCHAR(255),    
            OperatorWalletBaseUrl VARCHAR(255),
            WinFactor DECIMAL(5,4)                      
        );

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
    SessionId UUID PRIMARY KEY,  
    Token VARCHAR(255),         
    PlayerName VARCHAR(255),   
    Currency VARCHAR(10),       
    OperatorBaseUrl VARCHAR(255), 
    CreatedAt TIMESTAMP DEFAULT CURRENT_TIMESTAMP,  
    OperatorName VARCHAR(255),     
    OperatorGameName VARCHAR(255),  
    GameName VARCHAR(255)                   
);


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
    Platform VARCHAR(100) , 
    Operator VARCHAR(100) , 
    Client VARCHAR(255) ,       
    Game VARCHAR(100) ,         
    Status VARCHAR(100) ,               
    Description VARCHAR(600),           
    RoundID UUID,                       
    Timestamp TIMESTAMP  DEFAULT CURRENT_TIMESTAMP  
);