# Use plain JUnit5 for testing

* Status: accepted
* Deciders: Dana Example, Eli Example
* Date: 2019-04-23

Technical Story: https://example.org/issues/42

## Context and Problem Statement

We want to write unit tests in Java. Which test framework should we use?

## Considered Options

* [JUnit5](https://junit.org/junit5/)
* [TestNG](https://testng.org/)
* [Hamcrest](http://hamcrest.org/)

## Decision Outcome

Chosen option: "JUnit5", because it is the standard and the team knows it.

### Positive Consequences

* Tests run in every IDE

### Negative Consequences

* Parametrised tests need an extension
