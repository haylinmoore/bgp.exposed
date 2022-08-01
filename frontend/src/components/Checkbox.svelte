<script>
    export let label, checked;
</script>

<main on:click={() => checked = !checked}>
    <input type="checkbox" bind:checked>
    <span>{label}</span>
</main>

<style>
    main {
        display: flex;
        flex-direction: row;
        align-items: center;
        margin: 5px 0;
        cursor: pointer;
        -webkit-user-select: none;
        -moz-user-select: none;
        -ms-user-select: none;
        user-select: none;
    }

    span {
        margin-left: 14px;
        margin-top: 5px;
        margin-bottom: 5px;
    }

    input {
        -webkit-appearance: none;
        border: 2px solid white;
        padding: 10px;
        margin: 5px 0;
    }

    input:checked {
        background-color: red;
        border: none;
        padding: 12px;
    }
</style>
