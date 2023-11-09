**DAO** stands for Data Access Object. It is used to separate the data persistence logic in a separate layer, which is known as ***Separation of Logic***.

DAO pattern emphasises the low coupling between different components of an application. None of layers depend on it, but only `services` layer. (The most proper way would be using interfaces and not a concrete implementation. We're not using interfaces as there are no plans to replace implementation in foreseeable future.)

As the persistence logic is completely separate, it is much easier to write Unit tests for individual components. It is quite easy to mock data for an individual component of the application.

The DAO layer implementation resides in package `dao`, and correspondent mocks in package `mocks`. An example of a Unit test may be found in `pkg/services/dinosaurs_test.go`.
