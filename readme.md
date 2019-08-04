# MetaWat.ch Explorer
Внимание, это **НЕ стабильная и НЕ полная версия**

## Как это работает
В работе metawat.ch используются: Golang 1.12+, MySQL, MariaDB, Percona etc, ClickHouse v19+, NSQ 1.1.
 - Проект состоит из нескольких частей:
    + Explorer - отображает все данные, именно он работает на https://metawat.ch/
    - Воркер сбора транзакций
    - Воркер сбора данных о регистрации нод
    - Воркер обновления баланса адресов
    - Воркер обновления данных ноды
    - Воркер сбора данных о нодах из MetaGate
    - Воркер сбора данных о блоках
 - В MySQL хранятся данные о нодах и текущий балансах кошельков
 - ClickHouse хранит все транзакции и используется для построения графиков и ннекоторых списков, например делегаций
 - Explorer и воркеры общаются через очередь NSQ