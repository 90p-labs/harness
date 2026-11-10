# Visuals of a pull request body

Pick a visual for each kind of change. Each example has the main point line as it goes in a PR body, above the fence. Mark a state, flow, class or ER visual with `harness diagram`. A `sequenceDiagram` and a `journey` take no mark.

## State diagram

Use it for a state or a phase change.

A draft now goes to review, an approval merges it, and a rejection no longer sends it back to draft.

```mermaid
stateDiagram-v2
    [*] --> Draft
    Draft --> Review : request review (new)
    Review --> Merged : approve (changed)
    Review --> Draft : reject (removed)
```

## Flow chart

Use it for the steps of a command, a hook or a pipeline.

The command now checks its input before it stores, and the legacy step is gone.

```mermaid
flowchart TD
    classDef removed stroke:#e2001f,stroke-width:3px,color:#e2001f,stroke-dasharray:5 5
    cmd[Command] -->|"(new)"| check[Check input]
    check -->|"(changed)"| store[Store]
    legacy[Legacy]
    class legacy removed
    linkStyle 0 stroke:#00b34e,stroke-width:3px,color:#00b34e
    linkStyle 1 stroke:#e0a84a,stroke-width:3px,color:#e0a84a
```

## Sequence diagram

Use it for the calls between processes, an HTTP edge or a consumer, in time order.

The client posts an order, the API saves it in the store, and the client gets a 201 with the order id.

```mermaid
sequenceDiagram
    participant Client
    participant Api
    participant Store
    Client->>Api: POST /orders
    Api->>Store: save the order
    Store-->>Api: order id
    Api-->>Client: 201 with the order id
```

## Journey

Use it for the steps that a user takes in a flow, and how the user rates each step.

Creating an order takes four steps, and the user rates filling the fields lowest, at 3.

```mermaid
journey
    title Create an order
    section Prepare
      Open the form: 5: User
      Fill the fields: 3: User
    section Send
      Submit the form: 4: User
      Read the result: 5: User
```

## Class diagram

Use it for a change of a domain model or of a contract.

A new Order class holds many items, and the Item class changes to fit.

```mermaid
classDiagram
    classDef new stroke:#00b34e,stroke-width:3px,color:#00b34e
    classDef changed stroke:#e0a84a,stroke-width:3px,color:#e0a84a
    class Order
    class Item
    Order "1" --> "many" Item : (new)
    cssClass "Order" new
    cssClass "Item" changed
```

## ER diagram

Use it for a schema change: a table, a column or a relation.

An order now contains items, and an item gains a name and changes its id.

```mermaid
erDiagram
    ORDER ||--o{ ITEM : "contains (new)"
    ITEM {
        int id "(changed)"
        string name "(new)"
    }
```

## Wireframe

Use it for the layout of a UI change, as a boxed page or a CLI output, in a `text` fence.

The orders page lists each order with its item count and pages with Previous and Next.

```text
+------------------------------+
| Orders                  [New]|
+------------------------------+
| #1  Shipped        12 items  |
| #2  Pending         3 items  |
+------------------------------+
| < Previous           Next >  |
+------------------------------+
```
