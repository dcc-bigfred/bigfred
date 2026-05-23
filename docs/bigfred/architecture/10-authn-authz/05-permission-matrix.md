### 7a.5 Permission matrix

| Capability                                  | driver (own) | driver (leased) | signalman (idle) | signalman (active takeover) | admin |
|---------------------------------------------|:------------:|:---------------:|:----------------:|:---------------------------:|:-----:|
| Drive vehicle / train                       | ✅            | ✅               | ❌                | ✅                           | ❌¹    |
| Edit vehicle metadata, write CV             | ✅            | ❌               | ❌                | ❌                           | ❌¹    |
| Register vehicle (within own DCC pool)      | ✅            | n/a             | n/a              | n/a                         | ❌¹    |
| Create / edit train                         | ✅            | ❌               | ❌                | ❌                           | ❌¹    |
| Lease out a vehicle / train                 | ✅            | ❌               | ❌                | ❌                           | ❌¹    |
| Occupy an interlocking                      | ❌            | ❌               | ✅                | ✅                           | ❌¹    |
| Request takeover                            | ❌            | ❌               | ❌                | ✅²                          | ❌¹    |
| Manage users, roles, DCC pools              | ❌            | ❌               | ❌                | ❌                           | ✅     |

¹ `admin` is a management role only; if an admin also needs to drive,
   they must additionally hold the `driver` role (permanent or
   temporary).
² Takeover is only available to the signalman currently occupying an
   interlocking; idle signalmen do not have this power.
