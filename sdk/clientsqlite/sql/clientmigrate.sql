CREATE TABLE IF NOT EXISTS tmplog (
    Id INTEGER PRIMARY KEY,
    ExecutionIdentifier TEXT NOT NULL,
    ControllerName TEXT NOT NULL,
    Timestamp INTEGER NOT NULL,
    TemperatureInF TEXT NULL,
    DesiredTemperatureInF TEXT NULL,
    IsHeatingNotCooling INTEGER NOT NULL,
    TurningOnNotOff INTEGER NOT NULL,
    HostsPipeSeparated TEXT NULL,
    HasBeenSentToServer INTEGER NOT NULL
);