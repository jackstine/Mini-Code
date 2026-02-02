"""
Fibonacci module with optimized calculation using memoization.
"""

def fib(n, memo=None):
    """
    Calculate the nth Fibonacci number using memoization for optimization.
    
    Args:
        n: The position in the Fibonacci sequence
        memo: Dictionary to store previously calculated values
    
    Returns:
        The nth Fibonacci number
    """
    if memo is None:
        memo = {}
    
    # Base cases
    if n <= 1:
        return n
    
    # Check if already calculated
    if n in memo:
        return memo[n]
    
    # Calculate and store in memo
    memo[n] = fib(n - 1, memo) + fib(n - 2, memo)
    return memo[n]
