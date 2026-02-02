"""
Main program that calls the Fibonacci function from fib_module.
"""

from fib_module import fib

def main():
    """Main function to run the program."""
    n = 16
    result = fib(n)
    print(f"The {n}th Fibonacci number is: {result}")

if __name__ == "__main__":
    main()
