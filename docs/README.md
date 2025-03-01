# PgxWrappy

[![Go Reference](https://pkg.go.dev/badge/github.com/Arlandaren/pgxWrappy.svg)](https://pkg.go.dev/github.com/Arlandaren/pgxWrappy)
[![Go Report Card](https://goreportcard.com/badge/github.com/Arlandaren/pgxWrappy)](https://goreportcard.com/report/github.com/Arlandaren/pgxWrappy)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Arlandaren/pgxWrappy)](https://golang.org/)
[![Issues](https://img.shields.io/github/issues/Arlandaren/pgxWrappy)](https://github.com/Arlandaren/pgxWrappy/issues)
[![GitHub Last Commit](https://img.shields.io/github/last-commit/Arlandaren/pgxWrappy)](https://github.com/Arlandaren/pgxWrappy/commits/main)
[![GitHub Contributors](https://img.shields.io/github/contributors/Arlandaren/pgxWrappy)](https://github.com/Arlandaren/pgxWrappy/graphs/contributors)
[![Repo Size](https://img.shields.io/github/repo-size/Arlandaren/pgxWrappy)](https://github.com/Arlandaren/pgxWrappy)
[![GitHub Stars](https://img.shields.io/github/stars/Arlandaren/pgxWrappy.svg?style=social&label=Star)](https://github.com/Arlandaren/pgxWrappy)
[![GitHub Forks](https://img.shields.io/github/forks/Arlandaren/pgxWrappy.svg?style=social&label=Fork)](https://github.com/Arlandaren/pgxWrappy)

### PostgreSQL Wrapper Library for Convenient Scanning of Nested Structures

This library provides a convenient wrapper around the [pgx](https://github.com/jackc/pgx) library developed by [Jack Christensen](https://github.com/jackc). It simplifies database interactions with PostgreSQL by allowing easy scanning of query results into nested Go structures and slices.

If you've ever encountered the inconvenience of scanning and retrieving lists with `pgx`, this tool allows you to fully enjoy the `pgx` library by simplifying these operations.

## Key Features

- **Easy Scanning into Nested Structures**: Automatically maps SQL query results to Go structs, including nested structs.
- **Convenient Handling of Slices**: Supports scanning multiple rows into slices of structs or pointers to structs.
- **Transaction Support**: Provides wrappers for transactional operations with methods for beginning, committing, and rolling back transactions.
- **Integration with pgx**: Built on top of the high-performance [pgx](https://github.com/jackc/pgx) PostgreSQL driver, leveraging its robust features and reliability.

And even Jacks has mentioned this lib [here](https://github.com/jackc/pgx?tab=readme-ov-file#httpsgithubcomarlandarenpgxwrappy:~:text=jackc/pgx%20driver.-,https%3A//github.com/Arlandaren/pgxWrappy,working%20with%20the%20pgx%20library%2C%20providing%20convenient%20scanning%20of%20nested%20structures.,-About).
---

## Table of Contents

- [PgxWrappy](#pgxwrappy)
    - [PostgreSQL Wrapper Library for Convenient Scanning of Nested Structures](#postgresql-wrapper-library-for-convenient-scanning-of-nested-structures)
  - [Key Features](#key-features)
  - [Table of Contents](#table-of-contents)
  - [Installation](#installation)
  - [Usage](#usage)
    - [import package](#import-package)
    - [Initializing the Wrapper](#initializing-the-wrapper)
    - [Executing Queries](#executing-queries)
    - [Scanning into Structs](#scanning-into-structs)
      - [Get Method](#get-method)
      - [Select Method](#select-method)
    - [Transactions](#transactions)
  - [Field Tag Naming](#field-tag-naming)
  - [Why Choose `pgx` and `pgxWrappy`](#why-choose-pgx-and-pgxwrappy)
    - [Brief Comparison with Other PostgreSQL Drivers](#brief-comparison-with-other-postgresql-drivers)
      - [`database/sql` Standard Library](#databasesql-standard-library)
      - [`pq` Driver](#pq-driver)
      - [`pgx` Driver](#pgx-driver)
    - [Conclusion](#conclusion)
  - [Contributing](#contributing)
  - [License](#license)

---
Documentation can be found [here](https://github.com/Arlandaren/pgxWrappy/wiki/Documentation).

---
## Installation

To use this library, you need to have Go installed and set up. Import the package into your project:

```bash
go get -u github.com/Arlandaren/pgxWrappy
```

---

## Usage

### import package
```go
import "github.com/Arlandaren/pgxWrappy/pkg/postgres"
```

### Initializing the Wrapper

First, you need to initialize a connection pool using `pgxpool` and then create a new wrapper instance.

```go
import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/Arlandaren/pgxWrappy/pkg/postgres"
)

func main() {
    ctx := context.Background()
    pool, err := pgxpool.New(ctx, "postgres://username:password@localhost:5432/database")
    if err != nil {
        // Handle error
    }
    dbWrapper := pgxwrappy.NewWrapper(pool)
    // Use dbWrapper for database operations
}
```

---

### Executing Queries

You can execute queries using the `QueryRow`, `Query`, and `Exec` methods, which are wrappers around the corresponding `pgx` methods.

```go
// Executing a query that returns a single row
row := dbWrapper.QueryRow(ctx, "SELECT id, name FROM users WHERE id=$1", userID)

// Executing a query that returns multiple rows
rows, err := dbWrapper.Query(ctx, "SELECT id, name FROM users")

// Executing a command that doesn't return rows
_, err := dbWrapper.Exec(ctx, "UPDATE users SET name=$1 WHERE id=$2", newName, userID)
```

---

### Scanning into Structs

#### Get Method

The `Get` method executes a query that is expected to return a single row and scans the result into a struct.

```go
type User struct {
    ID   int    `db:"id"`
    Name string `db:"name"`
}

var user User
err := dbWrapper.Get(ctx, &user, "SELECT id, name FROM users WHERE id=$1", userID)
if err != nil {
    // Handle error
}
// Use the 'user' struct
```

#### Select Method

The `Select` method executes a query that returns multiple rows and scans the results into a slice of structs.

```go
var users []User
err := dbWrapper.Select(ctx, &users, "SELECT id, name FROM users")
if err != nil {
    // Handle error
}
// Use the 'users' slice
```

---

### Transactions

You can perform transactional operations using the `Begin`, `BeginTx`, `Commit`, and `Rollback` methods.

```go
txWrapper, err := dbWrapper.Begin(ctx)
if err != nil {
    // Handle error
}

defer func() {
    if err != nil {
        txWrapper.Rollback(ctx)
    } else {
        txWrapper.Commit(ctx)
    }
}()

// Perform transactional operations using txWrapper
err = txWrapper.Exec(ctx, "UPDATE accounts SET balance=balance-$1 WHERE id=$2", amount, fromAccountID)
if err != nil {
    return err
}

err = txWrapper.Exec(ctx, "UPDATE accounts SET balance=balance+$1 WHERE id=$2", amount, toAccountID)
if err != nil {
    return err
}
```

---

## Field Tag Naming

**Note**: To ensure correct scanning of query results into your structs, it's important to use the `db` struct tags to match the column names in your database. The tags should correspond exactly to the column names or use appropriate mapping if the names differ.

```go
type User struct {
    ID        int    `db:"id"`
    FirstName string `db:"first_name"`
    LastName  string `db:"last_name"`
    Email     string `db:"email"`
}
```

For nested structs, the field tags are used to flatten the structure during scanning.

```go
type Address struct {
    Street  string `db:"street"`
    City    string `db:"city"`
    ZipCode string `db:"zip_code"`
}

type User struct {
    ID      int     `db:"id"`
    Name    string  `db:"name"`
    Address Address `db:"-"`
}
```

In your SQL query, you should alias the columns appropriately:

```sql
SELECT
    id,
    name,
    street,
    city,
    zip_code
FROM users
```

This ensures that the scanning process correctly maps the SQL result columns to the fields in your nested structs.


- It is also possible to give a tag `db:"address"`, then the expected columns in the query would be: `address_street`, `address_city`, `address_zip_code`

Use the way that suits your situataion.
---

## Why Choose `pgx` and `pgxWrappy`

When working with PostgreSQL in Go, developers have several driver options to choose from. **`pgx`** stands out among other drivers for several reasons, and **`pgxWrappy`** addresses some of the common inconveniences developers face.

### Brief Comparison with Other PostgreSQL Drivers

#### `database/sql` Standard Library

- **Description**: Go's standard library interface for SQL databases.
- **Pros**:
  - Familiar interface for Go developers.
  - Supports multiple database backends.
- **Cons**:
  - Requires driver-specific implementations for full PostgreSQL features.
  - Less performant due to generalized abstractions.
  - Limited support for PostgreSQL-specific data types and features.
  - **Inconvenient Scanning**: Requires manual scanning of rows into variables, leading to verbose and repetitive code.

#### `pq` Driver

- **Description**: Pure Go driver for PostgreSQL, compatible with `database/sql`.
- **Pros**:
  - Simple and reliable for basic operations.
  - Widely used and tested.
- **Cons**:
  - No longer actively developed with new features.
  - Limited performance optimizations.
  - Doesn't support advanced PostgreSQL features out of the box.
  - **Inefficient Scanning**: Similar to `database/sql`, scanning rows can be cumbersome and boilerplate-heavy.

#### `pgx` Driver

- **Description**: High-performance PostgreSQL driver and toolkit for Go, developed by [Jack Christensen](https://github.com/jackc).
- **Pros**:
  - **Best-in-Class Performance**: Optimized for speed and efficiency.
  - **Full PostgreSQL Feature Support**: Access to advanced data types and protocols.
  - **Active Development**: Regular updates and community support.
  - **Flexibility**: Can be used with or without `database/sql`.
- **Cons**:
  - Slightly steeper learning curve due to extensive features.
  - **Inconvenient Scanning of Nested Structures**: While `pgx` is powerful, scanning query results into complex nested structs or slices requires manual code and can be cumbersome.

### Conclusion

**`pgx` is the best option for Go developers working with PostgreSQL**, offering superior performance, comprehensive feature support, and active maintenance.

By using **`pgx`** in conjunction with **`pgxWrappy`**, you further enhance your development experience:

- **Ease of Development**: Simplified scanning of query results into nested structures without boilerplate code.
- **Enhanced Productivity**: Focus on application logic rather than handling complex data mappings.
- **High Performance**: Leverage `pgx`'s speed while enjoying a more convenient API.
- **Seamless Transactions**: Intuitive methods for managing database transactions.

---

**Example Usage with `pgxWrappy`:**

```go
import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/Arlandaren/pgxWrappy/pkg/postgres"
)

func main() {
    ctx := context.Background()
    pool, err := pgxpool.New(ctx, "postgres://username:password@localhost:5432/database")
    if err != nil {
        // Handle error
    }
    dbWrapper := pgxwrappy.NewWrapper(pool)
    // Use dbWrapper for database operations

    // And other methods see above.
}
```

---

By choosing `pgx` paired with `pgxWrappy`, you embrace the most efficient and developer-friendly tools for PostgreSQL in Go. This combination allows you to fully enjoy the capabilities of `pgx`, making your database interactions smoother and more effective.

---


If you've ever encountered the inconvenience of scanning and retrieving lists with `pgx`, this tool allows you to fully enjoy the `pgx` library by simplifying these operations. By focusing on convenient scanning of nested structures and slices, this library aims to make database operations in Go more straightforward and efficient. It addresses common pain points that developers face when dealing with database interactions, especially the boilerplate code required for scanning query results into complex data structures.

Using this library, you can reduce code redundancy, improve readability, and maintain high performance in your applications. It's an excellent choice for developers who need more than what the standard library offers but prefer to avoid the overhead of a full ORM.

---

**Note**: This library builds upon the [pgx](https://github.com/jackc/pgx) PostgreSQL driver for Go, developed by [Jack Christensen](https://github.com/jackc). Special thanks to him for creating and maintaining such a high-performance and feature-rich driver.

---
## Contributing

Contributions are welcome! If you find a bug or want to add a feature, please open an issue or submit a pull request on [GitHub](https://github.com/yourusername/pgxwrappy).

---

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

---

By integrating `pgxWrappy` into your projects, you streamline your database interactions and harness the full power of `pgx` with added convenience. Give it a try and experience more efficient and enjoyable database programming in Go, just take a [first step](#installation)

---
